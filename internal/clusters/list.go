package clusters

import (
	"context"
	"fmt"

	"github.com/k0rdent/mcp-k0rdent-server/internal/logging"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// ListClusters retrieves ClusterDeployment resources from the specified namespaces.
// Returns summaries with key metadata including template, ready status, and labels.
func (m *Manager) ListClusters(ctx context.Context, namespaces []string) ([]ClusterDeploymentSummary, error) {
	logger := logging.WithContext(ctx, m.logger)
	logger.Debug("listing cluster deployments", "namespace_count", len(namespaces))

	if len(namespaces) == 0 {
		logger.Warn("no namespaces provided for cluster deployment listing")
		return []ClusterDeploymentSummary{}, nil
	}

	var summaries []ClusterDeploymentSummary

	// Query each namespace
	for _, ns := range namespaces {
		logger.Debug("listing cluster deployments in namespace", "namespace", ns)

		list, err := m.dynamicClient.Resource(ClusterDeploymentsGVR).Namespace(ns).List(ctx, metav1.ListOptions{})
		if err != nil {
			logger.Error("failed to list cluster deployments in namespace",
				"namespace", ns,
				"error", err,
			)
			return nil, fmt.Errorf("list cluster deployments in namespace %s: %w", ns, err)
		}

		logger.Debug("found cluster deployments in namespace",
			"namespace", ns,
			"count", len(list.Items),
		)

		// Convert each ClusterDeployment to summary
		for _, item := range list.Items {
			summary := clusterDeploymentToSummary(&item)
			summaries = append(summaries, summary)
		}
	}

	logger.Info("cluster deployments listed",
		"count", len(summaries),
		"namespace_count", len(namespaces),
	)

	return summaries, nil
}

// clusterDeploymentToSummary extracts key fields from a ClusterDeployment CR.
func clusterDeploymentToSummary(obj *unstructured.Unstructured) ClusterDeploymentSummary {
	summary := ClusterDeploymentSummary{
		Name:      obj.GetName(),
		Namespace: obj.GetNamespace(),
		Labels:    obj.GetLabels(),
		CreatedAt: obj.GetCreationTimestamp().Time,
	}

	// Extract template reference from spec.template
	if template, found, err := unstructured.NestedString(obj.Object, "spec", "template"); err == nil && found {
		summary.Template = template
	}

	// Extract ready status from status.conditions
	summary.Ready = IsResourceReady(obj)

	return summary
}
