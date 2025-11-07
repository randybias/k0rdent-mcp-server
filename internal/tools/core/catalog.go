package core

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/yaml"

	"github.com/k0rdent/mcp-k0rdent-server/internal/catalog"
	"github.com/k0rdent/mcp-k0rdent-server/internal/runtime"
)

type catalogListTool struct {
	session *runtime.Session
	manager *catalog.Manager
}

type catalogListInput struct {
	App     string `json:"app,omitempty"`
	Refresh bool   `json:"refresh,omitempty"`
}

type catalogListResult struct {
	Entries []catalog.CatalogEntry `json:"entries"`
}

type catalogInstallTool struct {
	session *runtime.Session
	manager *catalog.Manager
}

type catalogInstallInput struct {
	App           string `json:"app"`
	Template      string `json:"template"`
	Version       string `json:"version"`
	Namespace     string `json:"namespace,omitempty"`
	AllNamespaces bool   `json:"all_namespaces,omitempty"`
}

type catalogInstallResult struct {
	Applied []string `json:"applied"`
	Status  string   `json:"status"`
}

type catalogDeleteServiceTemplateTool struct {
	session *runtime.Session
	manager *catalog.Manager
}

type catalogDeleteInput struct {
	App           string `json:"app"`
	Template      string `json:"template"`
	Version       string `json:"version"`
	Namespace     string `json:"namespace,omitempty"`
	AllNamespaces bool   `json:"all_namespaces,omitempty"`
}

type catalogDeleteResult struct {
	Deleted []string `json:"deleted"`
	Status  string   `json:"status"`
}

func registerCatalog(server *mcp.Server, session *runtime.Session, manager *catalog.Manager) error {
	if manager == nil {
		return fmt.Errorf("catalog manager is required")
	}

	listTool := &catalogListTool{session: session, manager: manager}
	mcp.AddTool(server, &mcp.Tool{
		Name:        "k0rdent.catalog.list",
		Description: "List available ServiceTemplates from the k0rdent catalog",
	}, listTool.list)

	installTool := &catalogInstallTool{session: session, manager: manager}
	mcp.AddTool(server, &mcp.Tool{
		Name:        "k0rdent.catalog.install_servicetemplate",
		Description: "Install a ServiceTemplate from the k0rdent catalog. In DEV_ALLOW_ANY mode (uses kubeconfig), installs to kcm-system by default. In OIDC_REQUIRED mode (uses bearer token), requires explicit namespace or all_namespaces flag.",
	}, installTool.install)

	deleteTool := &catalogDeleteServiceTemplateTool{session: session, manager: manager}
	mcp.AddTool(server, &mcp.Tool{
		Name:        "k0rdent.catalog.delete_servicetemplate",
		Description: "Delete a ServiceTemplate and optionally its HelmRepository from k0rdent catalog. Follows same authentication modes as install (DEV_ALLOW_ANY, OIDC_REQUIRED). Returns success even if resource not found (idempotent).",
	}, deleteTool.delete)

	return nil
}

func (t *catalogListTool) list(ctx context.Context, req *mcp.CallToolRequest, input catalogListInput) (*mcp.CallToolResult, catalogListResult, error) {
	name := toolName(req)
	ctx, logger := toolContext(ctx, t.session, name, "tool.catalog")
	start := time.Now()

	logger.Debug("listing catalog entries", "tool", name, "app", input.App, "refresh", input.Refresh)

	entries, err := t.manager.List(ctx, input.App, input.Refresh)
	if err != nil {
		logger.Error("list catalog entries failed", "tool", name, "error", err)
		return nil, catalogListResult{}, fmt.Errorf("list catalog: %w", err)
	}

	logger.Info("catalog entries listed",
		"tool", name,
		"count", len(entries),
		"duration_ms", time.Since(start).Milliseconds(),
	)

	return nil, catalogListResult{Entries: entries}, nil
}

func (t *catalogInstallTool) install(ctx context.Context, req *mcp.CallToolRequest, input catalogInstallInput) (*mcp.CallToolResult, catalogInstallResult, error) {
	name := toolName(req)
	ctx, logger := toolContext(ctx, t.session, name, "tool.catalog")
	start := time.Now()

	logger.Debug("installing catalog template",
		"tool", name,
		"app", input.App,
		"template", input.Template,
		"version", input.Version,
		"namespace", input.Namespace,
		"all_namespaces", input.AllNamespaces,
	)

	// Validate required fields
	if input.App == "" {
		return nil, catalogInstallResult{}, fmt.Errorf("app is required")
	}
	if input.Template == "" {
		return nil, catalogInstallResult{}, fmt.Errorf("template is required")
	}
	if input.Version == "" {
		return nil, catalogInstallResult{}, fmt.Errorf("version is required")
	}

	// Resolve target namespaces
	targetNamespaces, err := t.resolveTargetNamespaces(ctx, input, logger)
	if err != nil {
		return nil, catalogInstallResult{}, err
	}

	logger.Debug("resolved target namespaces", "tool", name, "namespaces", targetNamespaces)

	// Get manifests from catalog
	manifests, err := t.manager.GetManifests(ctx, input.App, input.Template, input.Version)
	if err != nil {
		logger.Error("failed to get manifests", "tool", name, "error", err)
		return nil, catalogInstallResult{}, err
	}

	logger.Debug("manifests retrieved", "tool", name, "manifest_count", len(manifests))

	// Apply manifests to each target namespace
	var applied []string
	for _, targetNS := range targetNamespaces {
		logger.Debug("installing to namespace", "tool", name, "namespace", targetNS)

		for i, manifest := range manifests {
			// Parse YAML to unstructured
			obj := &unstructured.Unstructured{}
			if err := yaml.Unmarshal(manifest, &obj.Object); err != nil {
				logger.Error("failed to parse manifest", "tool", name, "manifest_index", i, "error", err)
				return nil, catalogInstallResult{}, fmt.Errorf("parse manifest %d: %w", i, err)
			}

			// Get GVK for processing
			gvk := obj.GroupVersionKind()

			// Convert v1alpha1 to v1beta1 if needed (catalog uses v1alpha1, clusters use v1beta1)
			if obj.GetAPIVersion() == "k0rdent.mirantis.com/v1alpha1" {
				obj.SetAPIVersion("k0rdent.mirantis.com/v1beta1")
				// Update gvk after version change
				gvk = obj.GroupVersionKind()
				logger.Debug("converted API version", "tool", name, "from", "v1alpha1", "to", "v1beta1")
			}

			// Set namespace to target namespace for ServiceTemplates and HelmRepositories
			if gvk.Kind == "ServiceTemplate" || gvk.Kind == "HelmRepository" {
				obj.SetNamespace(targetNS)
				logger.Debug("setting namespace", "tool", name, "kind", gvk.Kind, "namespace", targetNS)
			}

			// Determine GVR from GVK
			gvr := schema.GroupVersionResource{
				Group:    gvk.Group,
				Version:  gvk.Version,
				Resource: pluralize(gvk.Kind),
			}

			logger.Debug("applying resource",
				"tool", name,
				"kind", gvk.Kind,
				"name", obj.GetName(),
				"namespace", targetNS,
			)

			// Apply with server-side apply
			resourceClient := t.session.Clients.Dynamic.Resource(gvr).Namespace(targetNS)
			applyResult, err := resourceClient.Apply(ctx, obj.GetName(), obj, metav1.ApplyOptions{
				FieldManager: "k0rdent-mcp-server",
				Force:        true,
			})

			if err != nil {
				logger.Error("failed to apply resource",
					"tool", name,
					"kind", gvk.Kind,
					"name", obj.GetName(),
					"namespace", targetNS,
					"error", err,
				)
				return nil, catalogInstallResult{}, fmt.Errorf("apply %s %s in namespace %s: %w", gvk.Kind, obj.GetName(), targetNS, err)
			}

			resourceName := fmt.Sprintf("%s/%s/%s", targetNS, gvk.Kind, applyResult.GetName())
			applied = append(applied, resourceName)

			logger.Debug("resource applied",
				"tool", name,
				"kind", gvk.Kind,
				"name", applyResult.GetName(),
				"namespace", targetNS,
			)
		}
	}

	result := catalogInstallResult{
		Applied: applied,
		Status:  "created",
	}

	logger.Info("catalog template installed",
		"tool", name,
		"app", input.App,
		"template", input.Template,
		"version", input.Version,
		"applied_count", len(applied),
		"duration_ms", time.Since(start).Milliseconds(),
	)

	return nil, result, nil
}

func (t *catalogDeleteServiceTemplateTool) delete(ctx context.Context, req *mcp.CallToolRequest, input catalogDeleteInput) (*mcp.CallToolResult, catalogDeleteResult, error) {
	name := toolName(req)
	ctx, logger := toolContext(ctx, t.session, name, "tool.catalog")
	start := time.Now()

	logger.Debug("deleting catalog template",
		"tool", name,
		"app", input.App,
		"template", input.Template,
		"version", input.Version,
		"namespace", input.Namespace,
		"all_namespaces", input.AllNamespaces,
	)

	// Validate required fields
	if input.App == "" {
		return nil, catalogDeleteResult{}, fmt.Errorf("app is required")
	}
	if input.Template == "" {
		return nil, catalogDeleteResult{}, fmt.Errorf("template is required")
	}
	if input.Version == "" {
		return nil, catalogDeleteResult{}, fmt.Errorf("version is required")
	}

	// Resolve target namespaces using the same logic as install
	targetNamespaces, err := t.resolveTargetNamespaces(ctx, input, logger)
	if err != nil {
		return nil, catalogDeleteResult{}, err
	}

	logger.Debug("resolved target namespaces for deletion", "tool", name, "namespaces", targetNamespaces)

	// Get manifests from catalog to determine resource names
	manifests, err := t.manager.GetManifests(ctx, input.App, input.Template, input.Version)
	if err != nil {
		logger.Error("failed to get manifests", "tool", name, "error", err)
		return nil, catalogDeleteResult{}, err
	}

	logger.Debug("manifests retrieved for deletion", "tool", name, "manifest_count", len(manifests))

	// Delete resources from each target namespace
	var deleted []string
	var notFoundCount int

	for _, targetNS := range targetNamespaces {
		logger.Debug("deleting from namespace", "tool", name, "namespace", targetNS)

		for i, manifest := range manifests {
			// Parse YAML to unstructured
			obj := &unstructured.Unstructured{}
			if err := yaml.Unmarshal(manifest, &obj.Object); err != nil {
				logger.Error("failed to parse manifest", "tool", name, "manifest_index", i, "error", err)
				return nil, catalogDeleteResult{}, fmt.Errorf("parse manifest %d: %w", i, err)
			}

			// Get GVK for processing
			gvk := obj.GroupVersionKind()

			// Convert v1alpha1 to v1beta1 if needed (catalog uses v1alpha1, clusters use v1beta1)
			if obj.GetAPIVersion() == "k0rdent.mirantis.com/v1alpha1" {
				obj.SetAPIVersion("k0rdent.mirantis.com/v1beta1")
				gvk = obj.GroupVersionKind()
				logger.Debug("converted API version for deletion", "tool", name, "from", "v1alpha1", "to", "v1beta1")
			}

			// Only delete ServiceTemplates and HelmRepositories (namespace-scoped)
			if gvk.Kind != "ServiceTemplate" && gvk.Kind != "HelmRepository" {
				logger.Debug("skipping non-deletable resource", "tool", name, "kind", gvk.Kind)
				continue
			}

			// Determine GVR from GVK
			gvr := schema.GroupVersionResource{
				Group:    gvk.Group,
				Version:  gvk.Version,
				Resource: pluralize(gvk.Kind),
			}

			resourceName := obj.GetName()

			logger.Debug("deleting resource",
				"tool", name,
				"kind", gvk.Kind,
				"name", resourceName,
				"namespace", targetNS,
			)

			// Delete the resource
			resourceClient := t.session.Clients.Dynamic.Resource(gvr).Namespace(targetNS)
			err := resourceClient.Delete(ctx, resourceName, metav1.DeleteOptions{})

			if err != nil {
				// Check if error is NotFound - this is OK (idempotent)
				if strings.Contains(err.Error(), "not found") {
					logger.Debug("resource not found (already deleted)",
						"tool", name,
						"kind", gvk.Kind,
						"name", resourceName,
						"namespace", targetNS,
					)
					notFoundCount++
					continue
				}

				logger.Error("failed to delete resource",
					"tool", name,
					"kind", gvk.Kind,
					"name", resourceName,
					"namespace", targetNS,
					"error", err,
				)
				return nil, catalogDeleteResult{}, fmt.Errorf("delete %s %s in namespace %s: %w", gvk.Kind, resourceName, targetNS, err)
			}

			deletedResourceName := fmt.Sprintf("%s/%s/%s", targetNS, gvk.Kind, resourceName)
			deleted = append(deleted, deletedResourceName)

			logger.Debug("resource deleted",
				"tool", name,
				"kind", gvk.Kind,
				"name", resourceName,
				"namespace", targetNS,
			)
		}
	}

	// Determine status
	status := "deleted"
	if len(deleted) == 0 && notFoundCount > 0 {
		status = "not_found"
	}

	result := catalogDeleteResult{
		Deleted: deleted,
		Status:  status,
	}

	logger.Info("catalog template deleted",
		"tool", name,
		"app", input.App,
		"template", input.Template,
		"version", input.Version,
		"deleted_count", len(deleted),
		"not_found_count", notFoundCount,
		"duration_ms", time.Since(start).Milliseconds(),
	)

	return nil, result, nil
}

// resolveTargetNamespaces determines which namespace(s) to operate on for the delete tool
func (t *catalogDeleteServiceTemplateTool) resolveTargetNamespaces(ctx context.Context, input catalogDeleteInput, logger *slog.Logger) ([]string, error) {
	// If both namespace and all_namespaces are specified, return error
	if input.Namespace != "" && input.AllNamespaces {
		return nil, fmt.Errorf("cannot specify both 'namespace' and 'all_namespaces'")
	}

	// Case 1: Delete from all allowed namespaces
	if input.AllNamespaces {
		namespaces, err := t.getAllowedNamespaces(ctx, logger)
		if err != nil {
			return nil, fmt.Errorf("get allowed namespaces: %w", err)
		}
		if len(namespaces) == 0 {
			return nil, fmt.Errorf("no allowed namespaces found")
		}
		return namespaces, nil
	}

	// Case 2: Specific namespace provided
	if input.Namespace != "" {
		// Validate against namespace filter
		if t.session.NamespaceFilter != nil && !t.session.NamespaceFilter.MatchString(input.Namespace) {
			return nil, fmt.Errorf("namespace %q not allowed by namespace filter", input.Namespace)
		}
		return []string{input.Namespace}, nil
	}

	// Case 3: No namespace specified - determine default behavior
	// DEV_ALLOW_ANY mode (no filter or matches all): default to kcm-system
	// OIDC_REQUIRED mode (restricted filter): require explicit namespace
	if t.session.NamespaceFilter == nil || t.session.NamespaceFilter.MatchString("kcm-system") {
		// DEV_ALLOW_ANY mode - default to kcm-system
		logger.Debug("defaulting to kcm-system namespace (DEV_ALLOW_ANY mode)")
		return []string{"kcm-system"}, nil
	}

	// OIDC_REQUIRED mode - require explicit namespace
	return nil, fmt.Errorf("namespace must be specified in OIDC_REQUIRED mode (use 'namespace' parameter or 'all_namespaces: true')")
}

// getAllowedNamespaces returns all namespaces that match the namespace filter for delete operations
func (t *catalogDeleteServiceTemplateTool) getAllowedNamespaces(ctx context.Context, logger *slog.Logger) ([]string, error) {
	// List all namespaces from the cluster
	nsGVR := schema.GroupVersionResource{
		Group:    "",
		Version:  "v1",
		Resource: "namespaces",
	}

	nsList, err := t.session.Clients.Dynamic.Resource(nsGVR).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("list namespaces: %w", err)
	}

	var allowed []string
	for _, ns := range nsList.Items {
		nsName := ns.GetName()
		// If no filter, all namespaces are allowed
		if t.session.NamespaceFilter == nil {
			allowed = append(allowed, nsName)
		} else if t.session.NamespaceFilter.MatchString(nsName) {
			allowed = append(allowed, nsName)
		}
	}

	logger.Debug("found allowed namespaces", "count", len(allowed), "namespaces", allowed)
	return allowed, nil
}

// resolveTargetNamespaces determines which namespace(s) to install the ServiceTemplate into
func (t *catalogInstallTool) resolveTargetNamespaces(ctx context.Context, input catalogInstallInput, logger *slog.Logger) ([]string, error) {
	// If both namespace and all_namespaces are specified, return error
	if input.Namespace != "" && input.AllNamespaces {
		return nil, fmt.Errorf("cannot specify both 'namespace' and 'all_namespaces'")
	}

	// Case 1: Install to all allowed namespaces
	if input.AllNamespaces {
		namespaces, err := t.getAllowedNamespaces(ctx, logger)
		if err != nil {
			return nil, fmt.Errorf("get allowed namespaces: %w", err)
		}
		if len(namespaces) == 0 {
			return nil, fmt.Errorf("no allowed namespaces found")
		}
		return namespaces, nil
	}

	// Case 2: Specific namespace provided
	if input.Namespace != "" {
		// Validate against namespace filter
		if t.session.NamespaceFilter != nil && !t.session.NamespaceFilter.MatchString(input.Namespace) {
			return nil, fmt.Errorf("namespace %q not allowed by namespace filter", input.Namespace)
		}
		return []string{input.Namespace}, nil
	}

	// Case 3: No namespace specified - determine default behavior
	// DEV_ALLOW_ANY mode (no filter or matches all): default to kcm-system
	// OIDC_REQUIRED mode (restricted filter): require explicit namespace
	if t.session.NamespaceFilter == nil || t.session.NamespaceFilter.MatchString("kcm-system") {
		// DEV_ALLOW_ANY mode - default to kcm-system
		logger.Debug("defaulting to kcm-system namespace (DEV_ALLOW_ANY mode)")
		return []string{"kcm-system"}, nil
	}

	// OIDC_REQUIRED mode - require explicit namespace
	return nil, fmt.Errorf("namespace must be specified in OIDC_REQUIRED mode (use 'namespace' parameter or 'all_namespaces: true')")
}

// getAllowedNamespaces returns all namespaces that match the namespace filter
func (t *catalogInstallTool) getAllowedNamespaces(ctx context.Context, logger *slog.Logger) ([]string, error) {
	// List all namespaces from the cluster
	nsGVR := schema.GroupVersionResource{
		Group:    "",
		Version:  "v1",
		Resource: "namespaces",
	}

	nsList, err := t.session.Clients.Dynamic.Resource(nsGVR).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("list namespaces: %w", err)
	}

	var allowed []string
	for _, ns := range nsList.Items {
		nsName := ns.GetName()
		// If no filter, all namespaces are allowed
		if t.session.NamespaceFilter == nil {
			allowed = append(allowed, nsName)
		} else if t.session.NamespaceFilter.MatchString(nsName) {
			allowed = append(allowed, nsName)
		}
	}

	logger.Debug("found allowed namespaces", "count", len(allowed), "namespaces", allowed)
	return allowed, nil
}

// pluralize converts a Kubernetes Kind to its resource name (plural form).
// This is a simple implementation that handles most common cases.
func pluralize(kind string) string {
	lower := strings.ToLower(kind)

	// Special cases
	switch lower {
	case "endpoints":
		return lower
	case "componentstatus":
		return "componentstatuses"
	case "ingress":
		return "ingresses"
	}

	// Common pluralization rules
	if strings.HasSuffix(lower, "s") {
		return lower + "es"
	}
	if strings.HasSuffix(lower, "y") {
		return strings.TrimSuffix(lower, "y") + "ies"
	}

	return lower + "s"
}
