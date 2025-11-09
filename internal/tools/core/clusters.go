package core

import (
	"context"
	"fmt"
	"log/slog"
	"sort"
	"strings"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/yaml"

	"github.com/k0rdent/mcp-k0rdent-server/internal/clusters"
	"github.com/k0rdent/mcp-k0rdent-server/internal/k0rdent/api"
	"github.com/k0rdent/mcp-k0rdent-server/internal/metrics"
	"github.com/k0rdent/mcp-k0rdent-server/internal/runtime"
)

// Common defaults for cluster deployments
const (
	defaultControlPlaneNumber  = 3
	defaultWorkersNumber       = 2
	defaultAWSRootVolumeSize   = 32
	defaultAzureRootVolumeSize = 30
	defaultGCPRootVolumeSize   = 30
)

// validateAndDefaultNodeCounts validates and applies defaults to control plane and worker counts
func validateAndDefaultNodeCounts(controlPlaneNumber, workersNumber int) (int, int, error) {
	// Validate and default control plane number
	if controlPlaneNumber == 0 {
		controlPlaneNumber = defaultControlPlaneNumber
	} else if controlPlaneNumber < 0 {
		return 0, 0, fmt.Errorf("controlPlaneNumber must be at least 1 (got %d)", controlPlaneNumber)
	}

	// Validate and default workers number
	if workersNumber == 0 {
		workersNumber = defaultWorkersNumber
	} else if workersNumber < 0 {
		return 0, 0, fmt.Errorf("workersNumber must be at least 1 (got %d)", workersNumber)
	}

	return controlPlaneNumber, workersNumber, nil
}

// Tool handlers

type clustersListCredentialsTool struct {
	session *runtime.Session
}

type clustersListCredentialsInput struct {
	Namespace string `json:"namespace,omitempty"`
	Provider  string `json:"provider,omitempty"`
}

type clustersListCredentialsResult struct {
	Credentials []clusters.CredentialSummary `json:"credentials"`
}

type providersListTool struct {
	session *runtime.Session
}

type providersListResult struct {
	Providers []clusters.ProviderSummary `json:"providers"`
}

type providersListIdentitiesTool struct {
	session *runtime.Session
}

type providersListIdentitiesInput struct {
	Namespace string `json:"namespace,omitempty"`
}

type providersListIdentitiesResult struct {
	Identities []clusters.IdentitySummary `json:"identities"`
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

type clusterServiceApplyTool struct {
	session *runtime.Session
}

type clusterServiceApplyInput struct {
	ClusterNamespace  string                   `json:"clusterNamespace"`
	ClusterName       string                   `json:"clusterName"`
	TemplateNamespace string                   `json:"templateNamespace"`
	TemplateName      string                   `json:"templateName"`
	ServiceName       string                   `json:"serviceName,omitempty"`
	ServiceNamespace  string                   `json:"serviceNamespace,omitempty"`
	Values            map[string]any           `json:"values,omitempty"`
	ValuesFrom        []serviceValuesFromInput `json:"valuesFrom,omitempty"`
	HelmOptions       *serviceHelmOptionsInput `json:"helmOptions,omitempty"`
	DependsOn         []string                 `json:"dependsOn,omitempty"`
	Priority          *int64                   `json:"priority,omitempty"`
	ProviderConfig    map[string]any           `json:"providerConfig,omitempty"`
	DryRun            bool                     `json:"dryRun,omitempty"`
}

type serviceValuesFromInput struct {
	Kind     string `json:"kind"`
	Name     string `json:"name"`
	Key      string `json:"key"`
	Optional *bool  `json:"optional,omitempty"`
}

type serviceHelmOptionsInput struct {
	Atomic        *bool  `json:"atomic,omitempty"`
	Wait          *bool  `json:"wait,omitempty"`
	Timeout       string `json:"timeout,omitempty"`
	CleanupOnFail *bool  `json:"cleanupOnFail,omitempty"`
	Description   string `json:"description,omitempty"`
	DisableHooks  *bool  `json:"disableHooks,omitempty"`
	Replace       *bool  `json:"replace,omitempty"`
	SkipCRDs      *bool  `json:"skipCRDs,omitempty"`
	MaxHistory    *int64 `json:"maxHistory,omitempty"`
}

type clusterServiceApplyResult struct {
	Service          map[string]any   `json:"service"`
	Status           map[string]any   `json:"status,omitempty"`
	UpgradePaths     []map[string]any `json:"upgradePaths,omitempty"`
	ClusterName      string           `json:"clusterName"`
	ClusterNamespace string           `json:"clusterNamespace"`
	DryRun           bool             `json:"dryRun"`
}

var defaultProviderSummaries = []clusters.ProviderSummary{
	{Name: "aws", Title: "Amazon Web Services"},
	{Name: "azure", Title: "Microsoft Azure"},
	{Name: "gcp", Title: "Google Cloud Platform"},
	{Name: "vsphere", Title: "VMware vSphere"},
}

func registerClusters(server *mcp.Server, session *runtime.Session) error {
	// Register k0rdent.mgmt.providers.list
	providersTool := &providersListTool{session: session}
	mcp.AddTool(server, &mcp.Tool{
		Name:        "k0rdent.mgmt.providers.list",
		Description: "List supported infrastructure providers (e.g., AWS, Azure, Google Cloud, vSphere) available for credential onboarding.",
		Meta: mcp.Meta{
			"plane":    "mgmt",
			"category": "providers",
			"action":   "list",
		},
	}, providersTool.list)

	// Register k0rdent.mgmt.providers.listCredentials
	listCredsTool := &clustersListCredentialsTool{session: session}
	mcp.AddTool(server, &mcp.Tool{
		Name:        "k0rdent.mgmt.providers.listCredentials",
		Description: "List available Credentials for a given provider. Returns credentials from kcm-system (global) plus namespaces allowed by the current session.",
		Meta: mcp.Meta{
			"plane":    "mgmt",
			"category": "providers",
			"action":   "listCredentials",
		},
	}, listCredsTool.list)

	// Register k0rdent.mgmt.providers.listIdentities
	identitiesTool := &providersListIdentitiesTool{session: session}
	mcp.AddTool(server, &mcp.Tool{
		Name:        "k0rdent.mgmt.providers.listIdentities",
		Description: "List ClusterIdentity resources referenced by Credentials, including provider metadata and associated credentials.",
		Meta: mcp.Meta{
			"plane":    "mgmt",
			"category": "providers",
			"action":   "listIdentities",
		},
	}, identitiesTool.list)

	// Register k0rdent.mgmt.clusterTemplates.list
	listTemplsTool := &clustersListTemplatesTool{session: session}
	mcp.AddTool(server, &mcp.Tool{
		Name:        "k0rdent.mgmt.clusterTemplates.list",
		Description: "List available ClusterTemplates. Differentiates global (kcm-system) vs local templates, enforcing namespace filters. Input scope: 'global', 'local', or 'all'.",
		Meta: mcp.Meta{
			"plane":    "mgmt",
			"category": "clusterTemplates",
			"action":   "list",
		},
	}, listTemplsTool.list)

	// Register k0rdent.mgmt.clusterDeployments.list
	listClustersTool := &clustersListTool{session: session}
	mcp.AddTool(server, &mcp.Tool{
		Name:        "k0rdent.mgmt.clusterDeployments.list",
		Description: "List all ClusterDeployments. Returns clusters from allowed namespaces with optional filtering by namespace.",
		Meta: mcp.Meta{
			"plane":    "mgmt",
			"category": "clusterDeployments",
			"action":   "list",
		},
	}, listClustersTool.list)

	// Register k0rdent.mgmt.clusterDeployments.services.apply
	serviceApplyTool := &clusterServiceApplyTool{session: session}
	mcp.AddTool(server, &mcp.Tool{
		Name:        "k0rdent.mgmt.clusterDeployments.services.apply",
		Description: "Attach or update a ServiceTemplate entry on a running ClusterDeployment using server-side apply. Supports dry-run previews and returns the service status snapshot.",
		Meta: mcp.Meta{
			"plane":    "mgmt",
			"category": "clusterDeployments",
			"action":   "services.apply",
		},
	}, serviceApplyTool.apply)

	// Register provider-specific cluster deployment tools

	// Register k0rdent.provider.aws.clusterDeployments.deploy
	awsDeployTool := &awsClusterDeployTool{session: session}
	mcp.AddTool(server, &mcp.Tool{
		Name:        "k0rdent.provider.aws.clusterDeployments.deploy",
		Description: "Deploy a new AWS Kubernetes cluster. Automatically selects the latest stable AWS template and validates AWS-specific configuration (region, instanceType). Exposes AWS-specific parameters directly in the tool schema for easy agent discovery.",
		Meta: mcp.Meta{
			"plane":    "provider",
			"category": "clusterDeployments",
			"action":   "deploy",
			"provider": "aws",
		},
	}, awsDeployTool.deploy)

	// Register k0rdent.provider.azure.clusterDeployments.deploy
	azureDeployTool := &azureClusterDeployTool{session: session}
	mcp.AddTool(server, &mcp.Tool{
		Name:        "k0rdent.provider.azure.clusterDeployments.deploy",
		Description: "Deploy a new Azure Kubernetes cluster. Automatically selects the latest stable Azure template and validates Azure-specific configuration (location, subscriptionID, vmSize). Exposes Azure-specific parameters directly in the tool schema for easy agent discovery.",
		Meta: mcp.Meta{
			"plane":    "provider",
			"category": "clusterDeployments",
			"action":   "deploy",
			"provider": "azure",
		},
	}, azureDeployTool.deploy)

	// Register k0rdent.provider.gcp.clusterDeployments.deploy
	gcpDeployTool := &gcpClusterDeployTool{session: session}
	mcp.AddTool(server, &mcp.Tool{
		Name:        "k0rdent.provider.gcp.clusterDeployments.deploy",
		Description: "Deploy a new GCP Kubernetes cluster. Automatically selects the latest stable GCP template and validates GCP-specific configuration (project, region, network.name, instanceType). Exposes GCP-specific parameters directly in the tool schema for easy agent discovery.",
		Meta: mcp.Meta{
			"plane":    "provider",
			"category": "clusterDeployments",
			"action":   "deploy",
			"provider": "gcp",
		},
	}, gcpDeployTool.deploy)

	// Register k0rdent.mgmt.clusterDeployments.delete
	deleteTool := &clustersDeleteTool{session: session}
	mcp.AddTool(server, &mcp.Tool{
		Name:        "k0rdent.mgmt.clusterDeployments.delete",
		Description: "Delete a ClusterDeployment. Uses foreground propagation to ensure proper finalizer execution and resource cleanup. By default (wait=false), returns immediately after initiating deletion. Set wait=true to poll until deletion completes. Idempotent (returns success if already deleted).",
		Meta: mcp.Meta{
			"plane":    "mgmt",
			"category": "clusterDeployments",
			"action":   "delete",
		},
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

	providerFilter := strings.ToLower(strings.TrimSpace(input.Provider))
	var filtered []clusters.CredentialSummary
	if providerFilter == "" {
		filtered = credentials
	} else {
		logger.Debug("filtering credentials by provider", "tool", name, "provider", providerFilter)
		filtered = make([]clusters.CredentialSummary, 0, len(credentials))
		for _, cred := range credentials {
			if strings.EqualFold(cred.Provider, providerFilter) {
				filtered = append(filtered, cred)
			}
		}
	}

	logger.Info("cluster credentials listed",
		"tool", name,
		"count", len(filtered),
		"duration_ms", time.Since(start).Milliseconds(),
	)

	return nil, clustersListCredentialsResult{Credentials: filtered}, nil
}

func (t *providersListTool) list(ctx context.Context, req *mcp.CallToolRequest, _ any) (*mcp.CallToolResult, providersListResult, error) {
	name := toolName(req)
	ctx, logger := toolContext(ctx, t.session, name, "tool.clusters")
	logger.Info("listing infrastructure providers", "tool", name, "count", len(defaultProviderSummaries))

	providers := make([]clusters.ProviderSummary, len(defaultProviderSummaries))
	copy(providers, defaultProviderSummaries)

	return nil, providersListResult{Providers: providers}, nil
}

func (t *providersListIdentitiesTool) list(ctx context.Context, req *mcp.CallToolRequest, input providersListIdentitiesInput) (*mcp.CallToolResult, providersListIdentitiesResult, error) {
	name := toolName(req)
	ctx, logger := toolContext(ctx, t.session, name, "tool.clusters")
	start := time.Now()

	credsHelper := &clustersListCredentialsTool{session: t.session}
	targetNamespaces, err := credsHelper.resolveTargetNamespaces(ctx, input.Namespace, logger)
	if err != nil {
		logger.Error("failed to resolve target namespaces for identities", "tool", name, "error", err)
		return nil, providersListIdentitiesResult{}, fmt.Errorf("resolve namespaces: %w", err)
	}

	logger.Debug("resolved namespaces for identity listing", "tool", name, "namespaces", targetNamespaces)

	identities, err := t.session.Clusters.ListIdentities(ctx, targetNamespaces)
	if err != nil {
		logger.Error("failed to list identities", "tool", name, "error", err)
		return nil, providersListIdentitiesResult{}, fmt.Errorf("list identities: %w", err)
	}

	for i := range identities {
		sort.Strings(identities[i].Credentials)
	}
	sort.Slice(identities, func(i, j int) bool {
		if identities[i].Namespace == identities[j].Namespace {
			return identities[i].Name < identities[j].Name
		}
		return identities[i].Namespace < identities[j].Namespace
	})

	logger.Info("cluster identities listed",
		"tool", name,
		"count", len(identities),
		"duration_ms", time.Since(start).Milliseconds(),
	)

	return nil, providersListIdentitiesResult{Identities: identities}, nil
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

		// Wait for deletion to complete using shared helper
		waitHelper := &clusterWaitHelper{session: t.session}
		completed, err := waitHelper.waitForDeletion(ctx, targetNamespace, input.Name, pollInterval, deletionTimeout, logger)
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

func (t *clusterServiceApplyTool) apply(ctx context.Context, req *mcp.CallToolRequest, input clusterServiceApplyInput) (*mcp.CallToolResult, clusterServiceApplyResult, error) {
	name := toolName(req)
	ctx, logger := toolContext(ctx, t.session, name, "tool.clusters")
	start := time.Now()
	outcome := metrics.OutcomeSuccess
	defer func() {
		if t.session != nil && t.session.ClusterMetrics != nil {
			t.session.ClusterMetrics.RecordServiceApply(outcome, time.Since(start))
		}
	}()

	clusterNamespace := strings.TrimSpace(input.ClusterNamespace)
	clusterName := strings.TrimSpace(input.ClusterName)
	templateNamespace := strings.TrimSpace(input.TemplateNamespace)
	templateName := strings.TrimSpace(input.TemplateName)
	serviceNamespace := strings.TrimSpace(input.ServiceNamespace)
	serviceName := strings.TrimSpace(input.ServiceName)
	if templateNamespace == "" {
		templateNamespace = t.session.GlobalNamespace()
	}

	if clusterNamespace == "" {
		outcome = metrics.OutcomeError
		return nil, clusterServiceApplyResult{}, fmt.Errorf("clusterNamespace is required")
	}
	if clusterName == "" {
		outcome = metrics.OutcomeError
		return nil, clusterServiceApplyResult{}, fmt.Errorf("clusterName is required")
	}
	if templateName == "" {
		outcome = metrics.OutcomeError
		return nil, clusterServiceApplyResult{}, fmt.Errorf("templateName is required")
	}
	if serviceName == "" {
		serviceName = templateName
	}
	if serviceName == "" {
		outcome = metrics.OutcomeError
		return nil, clusterServiceApplyResult{}, fmt.Errorf("serviceName could not be derived")
	}
	if err := t.ensureNamespaceAllowed("clusterNamespace", clusterNamespace); err != nil {
		outcome = metrics.OutcomeForbidden
		return nil, clusterServiceApplyResult{}, err
	}
	if err := t.ensureNamespaceAllowed("templateNamespace", templateNamespace); err != nil {
		outcome = metrics.OutcomeForbidden
		return nil, clusterServiceApplyResult{}, err
	}
	if serviceNamespace != "" {
		if err := t.ensureNamespaceAllowed("serviceNamespace", serviceNamespace); err != nil {
			outcome = metrics.OutcomeForbidden
			return nil, clusterServiceApplyResult{}, err
		}
	}

	logger.Debug("applying cluster service",
		"tool", name,
		"cluster_namespace", clusterNamespace,
		"cluster_name", clusterName,
		"template", fmt.Sprintf("%s/%s", templateNamespace, templateName),
		"service_name", serviceName,
		"service_namespace", serviceNamespace,
		"dry_run", input.DryRun,
	)

	if input.Priority != nil && *input.Priority < 0 {
		outcome = metrics.OutcomeError
		return nil, clusterServiceApplyResult{}, fmt.Errorf("priority must be zero or positive")
	}

	client := t.session.Clients.Dynamic

	clusterObj, err := client.
		Resource(api.ClusterDeploymentGVR()).
		Namespace(clusterNamespace).
		Get(ctx, clusterName, metav1.GetOptions{})
	if err != nil {
		outcome = classifyMetricsOutcome(err)
		logger.Error("failed to fetch cluster deployment", "tool", name, "error", err)
		return nil, clusterServiceApplyResult{}, fmt.Errorf("get cluster deployment: %w", err)
	}

	existingServices := collectServiceNames(clusterObj)

	var dependsOnPtr *[]string
	if len(input.DependsOn) > 0 {
		deps := make([]string, len(input.DependsOn))
		var missing []string
		for i, raw := range input.DependsOn {
			dep := strings.TrimSpace(raw)
			if dep == "" {
				outcome = metrics.OutcomeError
				return nil, clusterServiceApplyResult{}, fmt.Errorf("dependsOn[%d] must not be empty", i)
			}
			if dep == serviceName {
				outcome = metrics.OutcomeError
				return nil, clusterServiceApplyResult{}, fmt.Errorf("dependsOn cannot reference the target service (%s)", serviceName)
			}
			if _, ok := existingServices[dep]; !ok {
				missing = append(missing, dep)
			}
			deps[i] = dep
		}
		if len(missing) > 0 {
			outcome = metrics.OutcomeError
			return nil, clusterServiceApplyResult{}, fmt.Errorf("dependsOn references unknown services: %s", strings.Join(missing, ", "))
		}
		depsCopy := deps
		dependsOnPtr = &depsCopy
	}

	var valuesFromPtr *[]api.ClusterServiceValuesFrom
	if len(input.ValuesFrom) > 0 {
		ptr, err := convertValuesFromInputs(input.ValuesFrom)
		if err != nil {
			outcome = metrics.OutcomeError
			return nil, clusterServiceApplyResult{}, err
		}
		valuesFromPtr = ptr
	}

	helmOpts, err := convertHelmOptionsInput(input.HelmOptions)
	if err != nil {
		outcome = metrics.OutcomeError
		return nil, clusterServiceApplyResult{}, err
	}

	var serviceValues *string
	if len(input.Values) > 0 {
		valuesYAML, err := yaml.Marshal(input.Values)
		if err != nil {
			outcome = metrics.OutcomeError
			return nil, clusterServiceApplyResult{}, fmt.Errorf("encode values: %w", err)
		}
		val := string(valuesYAML)
		serviceValues = &val
	}

	templateObj, err := client.
		Resource(api.ServiceTemplateGVR()).
		Namespace(templateNamespace).
		Get(ctx, templateName, metav1.GetOptions{})
	if err != nil {
		outcome = classifyMetricsOutcome(err)
		logger.Error("service template validation failed", "tool", name, "error", err)
		return nil, clusterServiceApplyResult{}, fmt.Errorf("get service template: %w", err)
	}
	logger.Debug("validated service template",
		"tool", name,
		"template_namespace", templateObj.GetNamespace(),
		"template_name", templateObj.GetName(),
	)

	serviceSpec := api.ClusterServiceApplySpec{
		TemplateNamespace: templateNamespace,
		TemplateName:      templateName,
		ServiceName:       serviceName,
	}
	if serviceNamespace != "" {
		ns := serviceNamespace
		serviceSpec.ServiceNamespace = &ns
	}
	if serviceValues != nil {
		serviceSpec.Values = serviceValues
	}
	if valuesFromPtr != nil {
		serviceSpec.ValuesFrom = valuesFromPtr
	}
	if helmOpts != nil {
		serviceSpec.HelmOptions = helmOpts
	}
	if dependsOnPtr != nil {
		serviceSpec.DependsOn = dependsOnPtr
	}
	if input.Priority != nil {
		priority := *input.Priority
		serviceSpec.Priority = &priority
	}

	applyOpts := api.ApplyClusterServiceOptions{
		ClusterNamespace: clusterNamespace,
		ClusterName:      clusterName,
		DryRun:           input.DryRun,
		Service:          serviceSpec,
	}
	if len(input.ProviderConfig) > 0 {
		cfgCopy := make(map[string]any, len(input.ProviderConfig))
		for k, v := range input.ProviderConfig {
			cfgCopy[k] = v
		}
		applyOpts.ProviderConfig = &cfgCopy
	}

	applyResult, err := api.ApplyClusterService(ctx, client, applyOpts)
	if err != nil {
		outcome = metrics.OutcomeError
		logger.Error("failed to apply service", "tool", name, "error", err)
		return nil, clusterServiceApplyResult{}, err
	}

	statusSource := applyResult.Cluster
	if !input.DryRun {
		refreshed, err := client.
			Resource(api.ClusterDeploymentGVR()).
			Namespace(clusterNamespace).
			Get(ctx, clusterName, metav1.GetOptions{})
		if err != nil {
			logger.Warn("failed to refresh cluster after apply", "tool", name, "error", err)
		} else {
			statusSource = refreshed
		}
	}

	response := clusterServiceApplyResult{
		Service:          applyResult.Service,
		ClusterName:      clusterName,
		ClusterNamespace: clusterNamespace,
		DryRun:           input.DryRun,
	}

	appliedServiceName := serviceName
	if applyResult.Service != nil {
		if name, ok := applyResult.Service["name"].(string); ok && name != "" {
			appliedServiceName = name
		}
	}
	response.Status = extractServiceStatus(statusSource, appliedServiceName)
	response.UpgradePaths = extractServiceUpgradePaths(statusSource, appliedServiceName)

	statusState := ""
	if response.Status != nil {
		if state, ok := response.Status["state"].(string); ok {
			statusState = state
		}
	}

	logger.Info("cluster service apply completed",
		"tool", name,
		"cluster_namespace", clusterNamespace,
		"cluster_name", clusterName,
		"service_name", appliedServiceName,
		"dry_run", input.DryRun,
		"status_state", statusState,
		"duration_ms", time.Since(start).Milliseconds(),
	)

	return nil, response, nil
}

func (t *clusterServiceApplyTool) ensureNamespaceAllowed(field, namespace string) error {
	if namespace == "" {
		return fmt.Errorf("%s is required", field)
	}
	if t.session == nil || t.session.NamespaceFilter == nil || t.session.IsDevMode() {
		return nil
	}
	if t.session.NamespaceFilter.MatchString(namespace) {
		return nil
	}
	return fmt.Errorf("%s %q not allowed by namespace filter", field, namespace)
}

func collectServiceNames(cluster *unstructured.Unstructured) map[string]struct{} {
	names := make(map[string]struct{})
	if cluster == nil {
		return names
	}
	list, found, err := unstructured.NestedSlice(cluster.Object, "spec", "serviceSpec", "services")
	if err != nil || !found {
		return names
	}
	for _, entry := range list {
		if m, ok := entry.(map[string]any); ok {
			if name, ok := m["name"].(string); ok && name != "" {
				names[name] = struct{}{}
			}
		}
	}
	return names
}

func convertValuesFromInputs(list []serviceValuesFromInput) (*[]api.ClusterServiceValuesFrom, error) {
	entries := make([]api.ClusterServiceValuesFrom, len(list))
	for i, src := range list {
		kind := strings.TrimSpace(src.Kind)
		switch strings.ToLower(kind) {
		case "configmap":
			kind = "ConfigMap"
		case "secret":
			kind = "Secret"
		default:
			return nil, fmt.Errorf("valuesFrom[%d].kind must be ConfigMap or Secret", i)
		}
		name := strings.TrimSpace(src.Name)
		if name == "" {
			return nil, fmt.Errorf("valuesFrom[%d].name is required", i)
		}
		key := strings.TrimSpace(src.Key)
		if key == "" {
			return nil, fmt.Errorf("valuesFrom[%d].key is required", i)
		}
		entries[i] = api.ClusterServiceValuesFrom{
			Kind:     kind,
			Name:     name,
			Key:      key,
			Optional: src.Optional,
		}
	}
	return &entries, nil
}

func convertHelmOptionsInput(input *serviceHelmOptionsInput) (*api.ClusterServiceHelmOptions, error) {
	if input == nil {
		return nil, nil
	}
	if input.Timeout != "" {
		if _, err := time.ParseDuration(input.Timeout); err != nil {
			return nil, fmt.Errorf("helmOptions.timeout must be a valid duration: %w", err)
		}
	}
	if input.MaxHistory != nil && *input.MaxHistory < 0 {
		return nil, fmt.Errorf("helmOptions.maxHistory must be zero or positive")
	}
	return &api.ClusterServiceHelmOptions{
		Atomic:        input.Atomic,
		Wait:          input.Wait,
		Timeout:       input.Timeout,
		CleanupOnFail: input.CleanupOnFail,
		Description:   input.Description,
		DisableHooks:  input.DisableHooks,
		Replace:       input.Replace,
		SkipCRDs:      input.SkipCRDs,
		MaxHistory:    input.MaxHistory,
	}, nil
}

func extractServiceStatus(cluster *unstructured.Unstructured, serviceName string) map[string]any {
	if cluster == nil || serviceName == "" {
		return nil
	}
	list, found, err := unstructured.NestedSlice(cluster.Object, "status", "services")
	if err != nil || !found {
		return nil
	}
	for _, entry := range list {
		m, ok := entry.(map[string]any)
		if !ok {
			continue
		}
		if name, _ := m["name"].(string); name == serviceName {
			return deepCopyJSONMap(m)
		}
	}
	return nil
}

func extractServiceUpgradePaths(cluster *unstructured.Unstructured, serviceName string) []map[string]any {
	if cluster == nil || serviceName == "" {
		return nil
	}
	list, found, err := unstructured.NestedSlice(cluster.Object, "status", "servicesUpgradePaths")
	if err != nil || !found {
		return nil
	}
	var matches []map[string]any
	for _, entry := range list {
		m, ok := entry.(map[string]any)
		if !ok {
			continue
		}
		name, _ := m["name"].(string)
		if name == "" {
			if alt, ok := m["serviceName"].(string); ok {
				name = alt
			}
		}
		if name == serviceName {
			matches = append(matches, deepCopyJSONMap(m))
		}
	}
	if len(matches) == 0 {
		return nil
	}
	return matches
}

func deepCopyJSONMap(value map[string]any) map[string]any {
	if value == nil {
		return nil
	}
	copy := make(map[string]any, len(value))
	for k, v := range value {
		copy[k] = cloneJSONValue(v)
	}
	return copy
}

func cloneJSONValue(val any) any {
	switch v := val.(type) {
	case map[string]any:
		return deepCopyJSONMap(v)
	case []any:
		cp := make([]any, len(v))
		for i := range v {
			cp[i] = cloneJSONValue(v[i])
		}
		return cp
	default:
		return v
	}
}

func classifyMetricsOutcome(err error) string {
	switch {
	case apierrors.IsNotFound(err):
		return metrics.OutcomeNotFound
	case apierrors.IsForbidden(err):
		return metrics.OutcomeForbidden
	default:
		return metrics.OutcomeError
	}
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
