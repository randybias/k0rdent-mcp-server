package clusters

import (
	"context"
	"fmt"
	"strings"

	"github.com/k0rdent/mcp-k0rdent-server/internal/logging"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// DeleteCluster removes a ClusterDeployment resource using foreground propagation.
// This ensures finalizers execute properly and child resources are cleaned up.
// Returns idempotent result - success even if resource is already deleted.
func (m *Manager) DeleteCluster(ctx context.Context, namespace, name string) (DeleteResult, error) {
	logger := logging.WithContext(ctx, m.logger)
	logger.Info("deleting cluster",
		"name", name,
		"namespace", namespace,
	)

	// Validate inputs
	if name == "" {
		logger.Warn("cluster name is required for deletion")
		return DeleteResult{}, fmt.Errorf("%w: name is required", ErrInvalidRequest)
	}
	if namespace == "" {
		logger.Warn("namespace is required for deletion")
		return DeleteResult{}, fmt.Errorf("%w: namespace is required", ErrInvalidRequest)
	}

	// Check if resource exists
	_, err := m.dynamicClient.Resource(ClusterDeploymentsGVR).Namespace(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		// Check if error is NotFound - this is OK (idempotent)
		if isNotFoundError(err) {
			logger.Debug("cluster deployment not found (already deleted)",
				"name", name,
				"namespace", namespace,
			)
			return DeleteResult{
				Name:      name,
				Namespace: namespace,
				Status:    "not_found",
			}, nil
		}

		logger.Error("failed to get cluster deployment",
			"name", name,
			"namespace", namespace,
			"error", err,
		)
		return DeleteResult{}, fmt.Errorf("get cluster deployment: %w", err)
	}

	logger.Debug("cluster deployment exists, proceeding with deletion",
		"name", name,
		"namespace", namespace,
	)

	// Delete with foreground propagation policy
	propagationPolicy := metav1.DeletePropagationForeground
	deleteOptions := metav1.DeleteOptions{
		PropagationPolicy: &propagationPolicy,
	}

	err = m.dynamicClient.Resource(ClusterDeploymentsGVR).Namespace(namespace).Delete(ctx, name, deleteOptions)
	if err != nil {
		// Check again for NotFound (race condition)
		if isNotFoundError(err) {
			logger.Debug("cluster deployment deleted concurrently",
				"name", name,
				"namespace", namespace,
			)
			return DeleteResult{
				Name:      name,
				Namespace: namespace,
				Status:    "not_found",
			}, nil
		}

		logger.Error("failed to delete cluster deployment",
			"name", name,
			"namespace", namespace,
			"error", err,
		)
		return DeleteResult{}, fmt.Errorf("delete cluster deployment: %w", err)
	}

	result := DeleteResult{
		Name:      name,
		Namespace: namespace,
		Status:    "deleted",
	}

	logger.Info("cluster deletion initiated",
		"name", result.Name,
		"namespace", result.Namespace,
		"status", result.Status,
	)

	return result, nil
}

// isNotFoundError checks if an error indicates a resource was not found.
func isNotFoundError(err error) bool {
	if err == nil {
		return false
	}
	errMsg := err.Error()
	return strings.Contains(errMsg, "not found") ||
		strings.Contains(errMsg, "NotFound")
}
