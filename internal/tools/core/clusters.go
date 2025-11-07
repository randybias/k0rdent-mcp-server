package core

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"github.com/k0rdent/mcp-k0rdent-server/internal/clusters"
	"github.com/k0rdent/mcp-k0rdent-server/internal/runtime"
)

// Tool handlers

type clustersListCredentialsTool struct {
	session *runtime.Session
}

type clustersListCredentialsInput struct {
	Namespace string `json:"namespace,omitempty"`
}

type clustersListCredentialsResult struct {
	Credentials []clusters.CredentialSummary `json:"credentials"`
}

type clustersListTemplatesTool struct {
	session *runtime.Session
}

type clustersListTemplatesInput struct {
	Scope     string `json:"scope"`               // "global", "local", or "all"
	Namespace string `json:"namespace,omitempty"` // Optional namespace filter
}

type clustersListTemplatesResult struct {
	Templates []clusters.ClusterTemplateSummary `json:"templates"`
}

type clustersDeployTool struct {
	session *runtime.Session
}

type clustersDeployInput struct {
	Name             string            `json:"name"`
	Template         string            `json:"template"`
	Credential       string            `json:"credential"`
	Namespace        string            `json:"namespace,omitempty"`
	Labels           map[string]string `json:"labels,omitempty"`
	Config           map[string]any    `json:"config"`
	Wait             bool              `json:"wait,omitempty"`             // Wait for cluster to be ready before returning
	PollInterval     string            `json:"pollInterval,omitempty"`     // How often to check status (e.g. "30s"), default "30s"
	ProvisionTimeout string            `json:"provisionTimeout,omitempty"` // Max time to wait for provisioning (e.g. "30m"), default "30m"
	StallThreshold   string            `json:"stallThreshold,omitempty"`   // Warn if no progress for this duration (e.g. "10m"), default "10m"
}

type clustersDeployResult clusters.DeployResult

type clustersDeleteTool struct {
	session *runtime.Session
}

type clustersDeleteInput struct {
	Name            string `json:"name"`
	Namespace       string `json:"namespace,omitempty"`
	Wait            bool   `json:"wait,omitempty"`            // Wait for deletion to complete (default: false)
	PollInterval    string `json:"pollInterval,omitempty"`    // e.g. "60s", default "60s"
	DeletionTimeout string `json:"deletionTimeout,omitempty"` // e.g. "20m", default "20m"
}

type clustersDeleteResult clusters.DeleteResult

type clustersListTool struct {
	session *runtime.Session
}

type clustersListInput struct {
	Namespace string `json:"namespace,omitempty"`
}

type clustersListResult struct {
	Clusters []clusters.ClusterDeploymentSummary `json:"clusters"`
}

func registerClusters(server *mcp.Server, session *runtime.Session) error {
	// Register k0rdent.providers.listCredentials
	listCredsTool := &clustersListCredentialsTool{session: session}
	mcp.AddTool(server, &mcp.Tool{
		Name:        "k0rdent.providers.listCredentials",
		Description: "List available provider Credentials for cluster provisioning. Returns credentials from kcm-system (global) plus namespaces allowed by the current session. Credentials are tied to infrastructure providers (Azure, AWS, GCP, vSphere, etc.).",
	}, listCredsTool.list)

	// Register k0rdent.clusterTemplates.list
	listTemplsTool := &clustersListTemplatesTool{session: session}
	mcp.AddTool(server, &mcp.Tool{
		Name:        "k0rdent.clusterTemplates.list",
		Description: "List available ClusterTemplates. Differentiates global (kcm-system) vs local templates, enforcing namespace filters. Input scope: 'global', 'local', or 'all'.",
	}, listTemplsTool.list)

	// Register k0rdent.clusters.list
	listClustersTool := &clustersListTool{session: session}
	mcp.AddTool(server, &mcp.Tool{
		Name:        "k0rdent.clusters.list",
		Description: "List all ClusterDeployments. Returns clusters from allowed namespaces with optional filtering by namespace.",
	}, listClustersTool.list)

	// Register k0rdent.cluster.deploy
	deployTool := &clustersDeployTool{session: session}
	mcp.AddTool(server, &mcp.Tool{
		Name:        "k0rdent.cluster.deploy",
		Description: "Deploy a new ClusterDeployment using specified template and credential. In DEV_ALLOW_ANY mode, defaults to kcm-system namespace. In OIDC_REQUIRED mode, requires explicit namespace.",
	}, deployTool.deploy)

	// Register k0rdent.cluster.delete
	deleteTool := &clustersDeleteTool{session: session}
	mcp.AddTool(server, &mcp.Tool{
		Name:        "k0rdent.cluster.delete",
		Description: "Delete a ClusterDeployment. Uses foreground propagation to ensure proper finalizer execution and resource cleanup. By default (wait=false), returns immediately after initiating deletion. Set wait=true to poll until deletion completes. Idempotent (returns success if already deleted).",
	}, deleteTool.delete)

	return nil
}

func (t *clustersListCredentialsTool) list(ctx context.Context, req *mcp.CallToolRequest, input clustersListCredentialsInput) (*mcp.CallToolResult, clustersListCredentialsResult, error) {
	name := toolName(req)
	ctx, logger := toolContext(ctx, t.session, name, "tool.clusters")
	start := time.Now()

	// TODO: Add metrics tracking (task 2.3, 3.4)
	// Increment clusters_list_credentials_total counter
	// Record duration histogram on completion

	logger.Debug("listing cluster credentials",
		"tool", name,
		"namespace", input.Namespace,
	)

	// Resolve target namespaces
	targetNamespaces, err := t.resolveTargetNamespaces(ctx, input.Namespace, logger)
	if err != nil {
		logger.Error("failed to resolve target namespaces", "tool", name, "error", err)
		return nil, clustersListCredentialsResult{}, fmt.Errorf("resolve namespaces: %w", err)
	}

	logger.Debug("resolved target namespaces for credentials", "tool", name, "namespaces", targetNamespaces)

	// List credentials using cluster manager
	credentials, err := t.session.Clusters.ListCredentials(ctx, targetNamespaces)
	if err != nil {
		logger.Error("failed to list credentials", "tool", name, "error", err)
		return nil, clustersListCredentialsResult{}, fmt.Errorf("list credentials: %w", err)
	}

	logger.Info("cluster credentials listed",
		"tool", name,
		"count", len(credentials),
		"duration_ms", time.Since(start).Milliseconds(),
	)

	return nil, clustersListCredentialsResult{Credentials: credentials}, nil
}

func (t *clustersListTemplatesTool) list(ctx context.Context, req *mcp.CallToolRequest, input clustersListTemplatesInput) (*mcp.CallToolResult, clustersListTemplatesResult, error) {
	name := toolName(req)
	ctx, logger := toolContext(ctx, t.session, name, "tool.clusters")
	start := time.Now()

	// TODO: Add metrics tracking (task 2.3, 3.4)
	// Increment clusters_list_templates_total counter
	// Record duration histogram on completion

	logger.Debug("listing cluster templates",
		"tool", name,
		"scope", input.Scope,
		"namespace", input.Namespace,
	)

	// Validate scope
	if input.Scope != "global" && input.Scope != "local" && input.Scope != "all" {
		return nil, clustersListTemplatesResult{}, fmt.Errorf("scope must be 'global', 'local', or 'all'")
	}

	// Resolve target namespaces based on scope
	targetNamespaces, err := t.resolveTargetNamespaces(ctx, input.Scope, input.Namespace, logger)
	if err != nil {
		logger.Error("failed to resolve target namespaces", "tool", name, "error", err)
		return nil, clustersListTemplatesResult{}, fmt.Errorf("resolve namespaces: %w", err)
	}

	logger.Debug("resolved target namespaces for templates", "tool", name, "namespaces", targetNamespaces, "scope", input.Scope)

	// List templates using cluster manager
	templates, err := t.session.Clusters.ListTemplates(ctx, targetNamespaces)
	if err != nil {
		logger.Error("failed to list templates", "tool", name, "error", err)
		return nil, clustersListTemplatesResult{}, fmt.Errorf("list templates: %w", err)
	}

	logger.Info("cluster templates listed",
		"tool", name,
		"scope", input.Scope,
		"count", len(templates),
		"duration_ms", time.Since(start).Milliseconds(),
	)

	return nil, clustersListTemplatesResult{Templates: templates}, nil
}

func (t *clustersDeployTool) deploy(ctx context.Context, req *mcp.CallToolRequest, input clustersDeployInput) (*mcp.CallToolResult, clustersDeployResult, error) {
	name := toolName(req)
	ctx, logger := toolContext(ctx, t.session, name, "tool.clusters")
	start := time.Now()

	// TODO: Add metrics tracking (task 2.3, 3.4)
	// Increment clusters_deploy_total counter (label by outcome: success/error)
	// Record duration histogram on completion

	logger.Debug("deploying cluster",
		"tool", name,
		"cluster_name", input.Name,
		"template", input.Template,
		"credential", input.Credential,
		"namespace", input.Namespace,
	)

	// Validate required fields
	if input.Name == "" {
		return nil, clustersDeployResult{}, fmt.Errorf("cluster name is required")
	}
	if input.Template == "" {
		return nil, clustersDeployResult{}, fmt.Errorf("template is required")
	}
	if input.Credential == "" {
		return nil, clustersDeployResult{}, fmt.Errorf("credential is required")
	}
	if input.Config == nil || len(input.Config) == 0 {
		return nil, clustersDeployResult{}, fmt.Errorf("config is required")
	}

	// Resolve target namespace
	targetNamespace, err := t.resolveDeployNamespace(ctx, input.Namespace, logger)
	if err != nil {
		logger.Error("failed to resolve deploy namespace", "tool", name, "error", err)
		return nil, clustersDeployResult{}, fmt.Errorf("resolve namespace: %w", err)
	}

	logger.Debug("resolved deploy namespace", "tool", name, "namespace", targetNamespace)

	// Build deploy request
	deployReq := clusters.DeployRequest{
		Name:       input.Name,
		Template:   input.Template,
		Credential: input.Credential,
		Namespace:  input.Namespace,
		Labels:     input.Labels,
		Config:     input.Config,
	}

	// Deploy cluster using cluster manager
	deployResult, err := t.session.Clusters.DeployCluster(ctx, targetNamespace, deployReq)
	if err != nil {
		logger.Error("failed to deploy cluster", "tool", name, "error", err)
		return nil, clustersDeployResult{}, fmt.Errorf("deploy cluster: %w", err)
	}

	result := clustersDeployResult(deployResult)

	// If wait is requested, monitor the cluster until ready or timeout
	if input.Wait {
		logger.Info("waiting for cluster to be ready",
			"tool", name,
			"cluster_name", input.Name,
			"namespace", targetNamespace,
		)

		// Parse wait parameters with defaults
		pollInterval := 30 * time.Second
		if input.PollInterval != "" {
			if d, err := time.ParseDuration(input.PollInterval); err == nil {
				pollInterval = d
			} else {
				logger.Warn("invalid pollInterval, using default", "input", input.PollInterval, "default", pollInterval)
			}
		}

		provisionTimeout := 30 * time.Minute
		if input.ProvisionTimeout != "" {
			if d, err := time.ParseDuration(input.ProvisionTimeout); err == nil {
				provisionTimeout = d
			} else {
				logger.Warn("invalid provisionTimeout, using default", "input", input.ProvisionTimeout, "default", provisionTimeout)
			}
		}

		stallThreshold := 10 * time.Minute
		if input.StallThreshold != "" {
			if d, err := time.ParseDuration(input.StallThreshold); err == nil {
				stallThreshold = d
			} else {
				logger.Warn("invalid stallThreshold, using default", "input", input.StallThreshold, "default", stallThreshold)
			}
		}

		// Wait for cluster to be ready
		ready, err := t.waitForClusterReady(ctx, targetNamespace, input.Name, pollInterval, provisionTimeout, stallThreshold, logger)
		if err != nil {
			logger.Error("error while waiting for cluster", "tool", name, "error", err)
			return nil, clustersDeployResult{}, fmt.Errorf("wait for cluster ready: %w", err)
		}

		if !ready {
			logger.Warn("cluster did not become ready within timeout",
				"tool", name,
				"cluster_name", input.Name,
				"timeout", provisionTimeout,
			)
			return nil, clustersDeployResult{}, fmt.Errorf("cluster %s did not become ready within %v", input.Name, provisionTimeout)
		}

		logger.Info("cluster is ready",
			"tool", name,
			"cluster_name", input.Name,
			"namespace", targetNamespace,
		)
	}

	logger.Info("cluster deployment completed",
		"tool", name,
		"cluster_name", input.Name,
		"namespace", targetNamespace,
		"status", result.Status,
		"duration_ms", time.Since(start).Milliseconds(),
	)

	return nil, result, nil
}

// waitForClusterReady polls the ClusterDeployment until it becomes ready or times out
func (t *clustersDeployTool) waitForClusterReady(
	ctx context.Context,
	namespace string,
	name string,
	pollInterval time.Duration,
	timeout time.Duration,
	stallThreshold time.Duration,
	logger *slog.Logger,
) (bool, error) {
	startTime := time.Now()
	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()

	var lastConditionState string
	lastStateChange := time.Now()

	for {
		select {
		case <-ctx.Done():
			return false, ctx.Err()

		case <-ticker.C:
			// Check if we've exceeded the timeout
			if time.Since(startTime) > timeout {
				logger.Warn("cluster provisioning timeout exceeded",
					"cluster", name,
					"namespace", namespace,
					"timeout", timeout,
				)
				return false, nil
			}

			// Get current cluster status
			obj, err := t.session.Clients.Dynamic.Resource(clusters.ClusterDeploymentsGVR).
				Namespace(namespace).
				Get(ctx, name, metav1.GetOptions{})
			if err != nil {
				logger.Error("failed to get cluster status",
					"cluster", name,
					"namespace", namespace,
					"error", err,
				)
				return false, fmt.Errorf("get cluster status: %w", err)
			}

			// Check if cluster is ready
			if clusters.IsResourceReady(obj) {
				logger.Info("cluster is ready",
					"cluster", name,
					"namespace", namespace,
					"duration", time.Since(startTime),
				)
				return true, nil
			}

			// Extract current condition state for stall detection
			currentState := extractConditionState(obj)

			// Check for state changes (stall detection)
			if currentState != lastConditionState {
				logger.Debug("cluster state changed",
					"cluster", name,
					"namespace", namespace,
					"state", currentState,
				)
				lastConditionState = currentState
				lastStateChange = time.Now()
			} else {
				stallDuration := time.Since(lastStateChange)
				if stallDuration > stallThreshold {
					logger.Warn("no progress detected",
						"cluster", name,
						"namespace", namespace,
						"stall_duration", stallDuration,
						"state", currentState,
					)
				}
			}
		}
	}
}

// extractConditionState extracts a string representation of the current condition state
func extractConditionState(obj *unstructured.Unstructured) string {
	conditions, found, err := unstructured.NestedSlice(obj.Object, "status", "conditions")
	if err != nil || !found || len(conditions) == 0 {
		return "no-conditions"
	}

	// Find the most recent condition
	var latestCondition map[string]interface{}
	var latestTime time.Time

	for _, cond := range conditions {
		condMap, ok := cond.(map[string]interface{})
		if !ok {
			continue
		}

		timeStr, _, _ := unstructured.NestedString(condMap, "lastTransitionTime")
		if timeStr == "" {
			continue
		}

		t, err := time.Parse(time.RFC3339, timeStr)
		if err != nil {
			continue
		}

		if latestCondition == nil || t.After(latestTime) {
			latestCondition = condMap
			latestTime = t
		}
	}

	if latestCondition == nil {
		return "no-valid-conditions"
	}

	condType, _, _ := unstructured.NestedString(latestCondition, "type")
	status, _, _ := unstructured.NestedString(latestCondition, "status")
	reason, _, _ := unstructured.NestedString(latestCondition, "reason")
	message, _, _ := unstructured.NestedString(latestCondition, "message")

	return fmt.Sprintf("%s=%s reason=%s msg=%s", condType, status, reason, message)
}

// waitForDeletion polls the ClusterDeployment until it is deleted or times out
func (t *clustersDeleteTool) waitForDeletion(
	ctx context.Context,
	namespace string,
	name string,
	pollInterval time.Duration,
	timeout time.Duration,
	logger *slog.Logger,
) (bool, error) {
	startTime := time.Now()
	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return false, ctx.Err()

		case <-ticker.C:
			// Check if we've exceeded the timeout
			if time.Since(startTime) > timeout {
				logger.Warn("deletion timeout exceeded",
					"cluster", name,
					"namespace", namespace,
					"timeout", timeout,
				)
				return false, nil
			}

			// Check if cluster still exists
			_, err := t.session.Clients.Dynamic.Resource(clusters.ClusterDeploymentsGVR).
				Namespace(namespace).
				Get(ctx, name, metav1.GetOptions{})

			if err != nil {
				// Check if it's a NotFound error (cluster was deleted)
				if errors.IsNotFound(err) {
					logger.Info("cluster deleted successfully",
						"cluster", name,
						"namespace", namespace,
						"duration", time.Since(startTime),
					)
					return true, nil
				}
				// Other errors
				logger.Error("error checking cluster status during deletion",
					"cluster", name,
					"namespace", namespace,
					"error", err,
				)
				return false, fmt.Errorf("check cluster status: %w", err)
			}

			// Cluster still exists, log progress
			logger.Debug("cluster still exists, waiting for deletion",
				"cluster", name,
				"namespace", namespace,
				"elapsed", time.Since(startTime),
			)
		}
	}
}

func (t *clustersDeleteTool) delete(ctx context.Context, req *mcp.CallToolRequest, input clustersDeleteInput) (*mcp.CallToolResult, clustersDeleteResult, error) {
	name := toolName(req)
	ctx, logger := toolContext(ctx, t.session, name, "tool.clusters")
	start := time.Now()

	// TODO: Add metrics tracking (task 2.3, 3.4)
	// Increment clusters_delete_total counter (label by outcome: success/error)
	// Record duration histogram on completion

	logger.Debug("deleting cluster",
		"tool", name,
		"cluster_name", input.Name,
		"namespace", input.Namespace,
	)

	// Validate required fields
	if input.Name == "" {
		return nil, clustersDeleteResult{}, fmt.Errorf("cluster name is required")
	}

	// Resolve target namespace
	targetNamespace, err := t.resolveDeleteNamespace(ctx, input.Namespace, logger)
	if err != nil {
		logger.Error("failed to resolve delete namespace", "tool", name, "error", err)
		return nil, clustersDeleteResult{}, fmt.Errorf("resolve namespace: %w", err)
	}

	logger.Debug("resolved delete namespace", "tool", name, "namespace", targetNamespace)

	// Delete cluster using cluster manager
	deleteResult, err := t.session.Clusters.DeleteCluster(ctx, targetNamespace, input.Name)
	if err != nil {
		logger.Error("failed to delete cluster", "tool", name, "error", err)
		return nil, clustersDeleteResult{}, fmt.Errorf("delete cluster: %w", err)
	}

	result := clustersDeleteResult(deleteResult)

	// If wait=true, wait for deletion to complete
	if input.Wait {
		logger.Info("waiting for cluster deletion to complete",
			"tool", name,
			"cluster_name", input.Name,
			"namespace", targetNamespace,
		)

		// Parse wait parameters with defaults
		pollInterval := 60 * time.Second
		if input.PollInterval != "" {
			if parsed, err := time.ParseDuration(input.PollInterval); err == nil {
				pollInterval = parsed
			}
		}

		deletionTimeout := 20 * time.Minute
		if input.DeletionTimeout != "" {
			if parsed, err := time.ParseDuration(input.DeletionTimeout); err == nil {
				deletionTimeout = parsed
			}
		}

		// Wait for deletion to complete
		completed, err := t.waitForDeletion(ctx, targetNamespace, input.Name, pollInterval, deletionTimeout, logger)
		if err != nil {
			logger.Error("error waiting for deletion", "tool", name, "error", err)
			return nil, result, fmt.Errorf("wait for deletion: %w", err)
		}

		if !completed {
			logger.Warn("deletion timeout exceeded",
				"tool", name,
				"cluster_name", input.Name,
				"namespace", targetNamespace,
				"timeout", deletionTimeout,
			)
		} else {
			logger.Info("cluster deletion verified",
				"tool", name,
				"cluster_name", input.Name,
				"namespace", targetNamespace,
			)
		}
	}

	logger.Info("cluster deletion completed",
		"tool", name,
		"cluster_name", input.Name,
		"namespace", targetNamespace,
		"status", result.Status,
		"wait", input.Wait,
		"duration_ms", time.Since(start).Milliseconds(),
	)

	return nil, result, nil
}

func (t *clustersListTool) list(ctx context.Context, req *mcp.CallToolRequest, input clustersListInput) (*mcp.CallToolResult, clustersListResult, error) {
	name := toolName(req)
	ctx, logger := toolContext(ctx, t.session, name, "tool.clusters")
	start := time.Now()

	// TODO: Add metrics tracking (task 2.3, 3.4)
	// Increment clusters_list_total counter
	// Record duration histogram on completion

	logger.Debug("listing cluster deployments",
		"tool", name,
		"namespace", input.Namespace,
	)

	// Resolve target namespaces
	var targetNamespaces []string
	var err error

	if input.Namespace != "" {
		// Validate the specified namespace
		if t.session.NamespaceFilter != nil && !t.session.NamespaceFilter.MatchString(input.Namespace) {
			logger.Error("namespace not allowed by filter", "tool", name, "namespace", input.Namespace)
			return nil, clustersListResult{}, fmt.Errorf("namespace %q not allowed by namespace filter", input.Namespace)
		}
		targetNamespaces = []string{input.Namespace}
	} else {
		// Get all allowed namespaces
		targetNamespaces, err = getAllowedNamespacesHelper(ctx, t.session, logger)
		if err != nil {
			logger.Error("failed to resolve target namespaces", "tool", name, "error", err)
			return nil, clustersListResult{}, fmt.Errorf("resolve namespaces: %w", err)
		}
	}

	logger.Debug("resolved target namespaces for cluster deployments", "tool", name, "namespaces", targetNamespaces)

	// List cluster deployments using cluster manager
	clusters, err := t.session.Clusters.ListClusters(ctx, targetNamespaces)
	if err != nil {
		logger.Error("failed to list cluster deployments", "tool", name, "error", err)
		return nil, clustersListResult{}, fmt.Errorf("list cluster deployments: %w", err)
	}

	logger.Info("cluster deployments listed",
		"tool", name,
		"count", len(clusters),
		"duration_ms", time.Since(start).Milliseconds(),
	)

	return nil, clustersListResult{Clusters: clusters}, nil
}

// Namespace resolution helpers

// resolveTargetNamespaces determines which namespaces to query for credentials
func (t *clustersListCredentialsTool) resolveTargetNamespaces(ctx context.Context, namespace string, logger *slog.Logger) ([]string, error) {
	// If specific namespace provided, validate it and use it
	if namespace != "" {
		if t.session.NamespaceFilter != nil && !t.session.NamespaceFilter.MatchString(namespace) {
			return nil, fmt.Errorf("namespace %q not allowed by namespace filter", namespace)
		}
		return []string{namespace}, nil
	}

	// Otherwise, return all allowed namespaces including global namespace
	namespaces, err := t.getAllowedNamespaces(ctx, logger)
	if err != nil {
		return nil, fmt.Errorf("get allowed namespaces: %w", err)
	}

	// Always include global namespace (kcm-system) for credentials
	globalNS := "kcm-system"
	hasGlobal := false
	for _, ns := range namespaces {
		if ns == globalNS {
			hasGlobal = true
			break
		}
	}
	if !hasGlobal {
		namespaces = append([]string{globalNS}, namespaces...)
	}

	return namespaces, nil
}

// resolveTargetNamespaces determines which namespaces to query for templates based on scope
func (t *clustersListTemplatesTool) resolveTargetNamespaces(ctx context.Context, scope, namespace string, logger *slog.Logger) ([]string, error) {
	// If specific namespace provided, validate it and use it
	if namespace != "" {
		if t.session.NamespaceFilter != nil && !t.session.NamespaceFilter.MatchString(namespace) {
			return nil, fmt.Errorf("namespace %q not allowed by namespace filter", namespace)
		}
		return []string{namespace}, nil
	}

	// Handle scope-based namespace resolution
	switch scope {
	case "global":
		return []string{"kcm-system"}, nil

	case "local":
		namespaces, err := t.getAllowedNamespaces(ctx, logger)
		if err != nil {
			return nil, fmt.Errorf("get allowed namespaces: %w", err)
		}
		// Filter out global namespace
		var localNamespaces []string
		for _, ns := range namespaces {
			if ns != "kcm-system" {
				localNamespaces = append(localNamespaces, ns)
			}
		}
		return localNamespaces, nil

	case "all":
		namespaces, err := t.getAllowedNamespaces(ctx, logger)
		if err != nil {
			return nil, fmt.Errorf("get allowed namespaces: %w", err)
		}
		// Ensure global namespace is included
		globalNS := "kcm-system"
		hasGlobal := false
		for _, ns := range namespaces {
			if ns == globalNS {
				hasGlobal = true
				break
			}
		}
		if !hasGlobal {
			namespaces = append([]string{globalNS}, namespaces...)
		}
		return namespaces, nil

	default:
		return nil, fmt.Errorf("invalid scope: %s (must be 'global', 'local', or 'all')", scope)
	}
}

// resolveDeployNamespace determines the target namespace for cluster deployment
func (t *clustersDeployTool) resolveDeployNamespace(ctx context.Context, namespace string, logger *slog.Logger) (string, error) {
	// If specific namespace provided, validate it
	if namespace != "" {
		if t.session.NamespaceFilter != nil && !t.session.NamespaceFilter.MatchString(namespace) {
			return "", fmt.Errorf("namespace %q not allowed by namespace filter", namespace)
		}
		return namespace, nil
	}

	// No namespace specified - determine default behavior
	// DEV_ALLOW_ANY mode (no filter or matches all): default to kcm-system
	// OIDC_REQUIRED mode (restricted filter): require explicit namespace
	if t.session.NamespaceFilter == nil || t.session.NamespaceFilter.MatchString("kcm-system") {
		// DEV_ALLOW_ANY mode - default to kcm-system
		logger.Debug("defaulting to kcm-system namespace (DEV_ALLOW_ANY mode)")
		return "kcm-system", nil
	}

	// OIDC_REQUIRED mode - require explicit namespace
	return "", fmt.Errorf("namespace must be specified in OIDC_REQUIRED mode (use 'namespace' parameter)")
}

// resolveDeleteNamespace determines the target namespace for cluster deletion
func (t *clustersDeleteTool) resolveDeleteNamespace(ctx context.Context, namespace string, logger *slog.Logger) (string, error) {
	// Same logic as deploy
	if namespace != "" {
		if t.session.NamespaceFilter != nil && !t.session.NamespaceFilter.MatchString(namespace) {
			return "", fmt.Errorf("namespace %q not allowed by namespace filter", namespace)
		}
		return namespace, nil
	}

	// DEV_ALLOW_ANY mode: default to kcm-system
	// OIDC_REQUIRED mode: require explicit namespace
	if t.session.NamespaceFilter == nil || t.session.NamespaceFilter.MatchString("kcm-system") {
		logger.Debug("defaulting to kcm-system namespace (DEV_ALLOW_ANY mode)")
		return "kcm-system", nil
	}

	return "", fmt.Errorf("namespace must be specified in OIDC_REQUIRED mode (use 'namespace' parameter)")
}

// getAllowedNamespaces returns all namespaces that match the namespace filter
func (t *clustersListCredentialsTool) getAllowedNamespaces(ctx context.Context, logger *slog.Logger) ([]string, error) {
	return getAllowedNamespacesHelper(ctx, t.session, logger)
}

func (t *clustersListTemplatesTool) getAllowedNamespaces(ctx context.Context, logger *slog.Logger) ([]string, error) {
	return getAllowedNamespacesHelper(ctx, t.session, logger)
}

// getAllowedNamespacesHelper is a shared helper to get allowed namespaces
func getAllowedNamespacesHelper(ctx context.Context, session *runtime.Session, logger *slog.Logger) ([]string, error) {
	// List all namespaces from the cluster
	nsGVR := schema.GroupVersionResource{
		Group:    "",
		Version:  "v1",
		Resource: "namespaces",
	}

	nsList, err := session.Clients.Dynamic.Resource(nsGVR).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("list namespaces: %w", err)
	}

	var allowed []string
	for _, ns := range nsList.Items {
		nsName := ns.GetName()
		// If no filter, all namespaces are allowed
		if session.NamespaceFilter == nil {
			allowed = append(allowed, nsName)
		} else if session.NamespaceFilter.MatchString(nsName) {
			allowed = append(allowed, nsName)
		}
	}

	logger.Debug("found allowed namespaces", "count", len(allowed), "namespaces", allowed)
	return allowed, nil
}
