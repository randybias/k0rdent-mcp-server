package core

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/k0rdent/mcp-k0rdent-server/internal/clusters"
	"github.com/k0rdent/mcp-k0rdent-server/internal/runtime"
)

// clusterWaitHelper provides shared wait functionality for all cluster deployment tools
type clusterWaitHelper struct {
	session *runtime.Session
}

// waitForClusterReady polls the ClusterDeployment until it becomes ready or times out
func (h *clusterWaitHelper) waitForClusterReady(
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
			obj, err := h.session.Clients.Dynamic.Resource(clusters.ClusterDeploymentsGVR).
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

// waitForDeletion polls the ClusterDeployment until it is deleted or times out
func (h *clusterWaitHelper) waitForDeletion(
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
			_, err := h.session.Clients.Dynamic.Resource(clusters.ClusterDeploymentsGVR).
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

// resolveDeployNamespace determines the target namespace for cluster deployment
func resolveDeployNamespace(ctx context.Context, session *runtime.Session, namespace string, logger *slog.Logger) (string, error) {
	// If specific namespace provided, validate it
	if namespace != "" {
		if session.NamespaceFilter != nil && !session.NamespaceFilter.MatchString(namespace) {
			return "", fmt.Errorf("namespace %q not allowed by namespace filter", namespace)
		}
		return namespace, nil
	}

	// No namespace specified - determine default behavior
	// DEV_ALLOW_ANY mode (no filter or matches all): default to kcm-system
	// OIDC_REQUIRED mode (restricted filter): require explicit namespace
	if session.NamespaceFilter == nil || session.NamespaceFilter.MatchString("kcm-system") {
		// DEV_ALLOW_ANY mode - default to kcm-system
		logger.Debug("defaulting to kcm-system namespace (DEV_ALLOW_ANY mode)")
		return "kcm-system", nil
	}

	// OIDC_REQUIRED mode - require explicit namespace
	return "", fmt.Errorf("namespace must be specified in OIDC_REQUIRED mode (use 'namespace' parameter)")
}
