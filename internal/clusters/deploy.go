package clusters

import (
	"context"
	"fmt"

	"github.com/k0rdent/mcp-k0rdent-server/internal/logging"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

var (
	// ClusterDeploymentsGVR is the GroupVersionResource for ClusterDeployment CRs
	ClusterDeploymentsGVR = schema.GroupVersionResource{
		Group:    "k0rdent.mirantis.com",
		Version:  "v1beta1",
		Resource: "clusterdeployments",
	}
)

// DeployCluster creates or updates a ClusterDeployment using server-side apply.
// The namespace parameter specifies where to create the deployment.
// Template and Credential references are resolved according to namespace rules.
func (m *Manager) DeployCluster(ctx context.Context, namespace string, req DeployRequest) (DeployResult, error) {
	logger := logging.WithContext(ctx, m.logger)
	logger.Info("deploying cluster",
		"name", req.Name,
		"namespace", namespace,
		"template", req.Template,
		"credential", req.Credential,
	)

	// Validate request
	if req.Name == "" {
		logger.Warn("cluster name is required")
		return DeployResult{}, fmt.Errorf("%w: name is required", ErrInvalidRequest)
	}
	if req.Template == "" {
		logger.Warn("template is required")
		return DeployResult{}, fmt.Errorf("%w: template is required", ErrInvalidRequest)
	}
	if req.Credential == "" {
		logger.Warn("credential is required")
		return DeployResult{}, fmt.Errorf("%w: credential is required", ErrInvalidRequest)
	}

	// Resolve template reference (namespace/name or name)
	templateNS, templateName, err := m.ResolveResourceNamespace(ctx, req.Template, namespace)
	if err != nil {
		logger.Error("failed to resolve template namespace",
			"template", req.Template,
			"error", err,
		)
		return DeployResult{}, fmt.Errorf("resolve template namespace: %w", err)
	}

	// Verify template exists
	_, err = m.dynamicClient.Resource(ClusterTemplatesGVR).Namespace(templateNS).Get(ctx, templateName, metav1.GetOptions{})
	if err != nil {
		logger.Error("template not found",
			"template", templateName,
			"namespace", templateNS,
			"error", err,
		)
		return DeployResult{}, fmt.Errorf("template %s not found in namespace %s: %w", templateName, templateNS, err)
	}

	logger.Debug("resolved template",
		"name", templateName,
		"namespace", templateNS,
	)

	// Resolve credential reference
	credentialNS, credentialName, err := m.ResolveResourceNamespace(ctx, req.Credential, namespace)
	if err != nil {
		logger.Error("failed to resolve credential namespace",
			"credential", req.Credential,
			"error", err,
		)
		return DeployResult{}, fmt.Errorf("resolve credential namespace: %w", err)
	}

	// Verify credential exists
	_, err = m.dynamicClient.Resource(CredentialsGVR).Namespace(credentialNS).Get(ctx, credentialName, metav1.GetOptions{})
	if err != nil {
		logger.Error("credential not found",
			"credential", credentialName,
			"namespace", credentialNS,
			"error", err,
		)
		return DeployResult{}, fmt.Errorf("credential %s not found in namespace %s: %w", credentialName, credentialNS, err)
	}

	logger.Debug("resolved credential",
		"name", credentialName,
		"namespace", credentialNS,
	)

	// Validate configuration for known cloud providers
	validationResult := ValidateConfig(templateName, req.Config)
	if !validationResult.IsValid() {
		logger.Warn("cluster configuration validation failed",
			"provider", validationResult.Provider,
			"errors", validationResult.Errors,
		)

		// Format error message based on provider
		var errorMsg string
		switch validationResult.Provider {
		case ProviderAWS:
			errorMsg = FormatAWSValidationError(validationResult.Errors)
		case ProviderAzure:
			errorMsg = FormatAzureValidationError(validationResult.Errors)
		case ProviderGCP:
			errorMsg = FormatGCPValidationError(validationResult.Errors)
		default:
			// Generic format for unknown providers (shouldn't reach here)
			errorMsg = "Configuration validation failed"
			for _, err := range validationResult.Errors {
				errorMsg += fmt.Sprintf("\n  - %s: %s", err.Field, err.Message)
			}
		}

		return DeployResult{}, fmt.Errorf("%w: %s", ErrInvalidRequest, errorMsg)
	}

	// Log successful validation if provider was detected
	if validationResult.Provider != ProviderUnknown {
		logger.Debug("configuration validation passed",
			"provider", validationResult.Provider,
		)
	}

	// Build ClusterDeployment manifest
	deployment := m.buildClusterDeployment(req, namespace, templateNS, templateName, credentialNS, credentialName)

	logger.Debug("applying cluster deployment",
		"name", req.Name,
		"namespace", namespace,
		"field_owner", m.fieldOwner,
	)

	// Apply using server-side apply
	result, err := m.dynamicClient.Resource(ClusterDeploymentsGVR).Namespace(namespace).Apply(
		ctx,
		req.Name,
		deployment,
		metav1.ApplyOptions{
			FieldManager: m.fieldOwner,
			Force:        true,
		},
	)
	if err != nil {
		logger.Error("failed to apply cluster deployment",
			"name", req.Name,
			"namespace", namespace,
			"error", err,
		)
		return DeployResult{}, fmt.Errorf("apply cluster deployment: %w", err)
	}

	// Check if this was a create or update by inspecting resource version
	status := "created"
	if result.GetResourceVersion() != "1" {
		status = "updated"
	}

	deployResult := DeployResult{
		Name:      result.GetName(),
		Namespace: result.GetNamespace(),
		UID:       string(result.GetUID()),
		Status:    status,
	}

	logger.Info("cluster deployment successful",
		"name", deployResult.Name,
		"namespace", deployResult.Namespace,
		"uid", deployResult.UID,
		"status", deployResult.Status,
	)

	return deployResult, nil
}

// buildClusterDeployment constructs the ClusterDeployment manifest.
func (m *Manager) buildClusterDeployment(
	req DeployRequest,
	namespace string,
	templateNS string,
	templateName string,
	credentialNS string,
	credentialName string,
) *unstructured.Unstructured {
	// Base labels
	labels := map[string]interface{}{
		"k0rdent.mirantis.com/managed": "true",
	}

	// Merge user-provided labels
	for k, v := range req.Labels {
		labels[k] = v
	}

	// Build manifest structure
	obj := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "k0rdent.mirantis.com/v1beta1",
			"kind":       "ClusterDeployment",
			"metadata": map[string]interface{}{
				"name":      req.Name,
				"namespace": namespace,
				"labels":    labels,
			},
			"spec": map[string]interface{}{
				"template": templateName,
				"credential": credentialName,
				"config": req.Config,
			},
		},
	}

	return obj
}
