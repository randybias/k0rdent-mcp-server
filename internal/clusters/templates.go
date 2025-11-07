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
	// ClusterTemplatesGVR is the GroupVersionResource for ClusterTemplate CRs
	ClusterTemplatesGVR = schema.GroupVersionResource{
		Group:    "k0rdent.mirantis.com",
		Version:  "v1beta1",
		Resource: "clustertemplates",
	}
)

// ListTemplates retrieves ClusterTemplate resources from the specified namespaces.
// Returns summaries with key metadata including description, provider, cloud tags, and version.
func (m *Manager) ListTemplates(ctx context.Context, namespaces []string) ([]ClusterTemplateSummary, error) {
	logger := logging.WithContext(ctx, m.logger)
	logger.Debug("listing cluster templates", "namespace_count", len(namespaces))

	if len(namespaces) == 0 {
		logger.Warn("no namespaces provided for template listing")
		return []ClusterTemplateSummary{}, nil
	}

	var summaries []ClusterTemplateSummary

	// Query each namespace
	for _, ns := range namespaces {
		logger.Debug("listing templates in namespace", "namespace", ns)

		list, err := m.dynamicClient.Resource(ClusterTemplatesGVR).Namespace(ns).List(ctx, metav1.ListOptions{})
		if err != nil {
			logger.Error("failed to list templates in namespace",
				"namespace", ns,
				"error", err,
			)
			return nil, fmt.Errorf("list templates in namespace %s: %w", ns, err)
		}

		logger.Debug("found templates in namespace",
			"namespace", ns,
			"count", len(list.Items),
		)

		// Convert each template to summary
		for _, item := range list.Items {
			summary, err := m.templateToSummary(&item)
			if err != nil {
				logger.Warn("failed to convert template to summary",
					"namespace", ns,
					"name", item.GetName(),
					"error", err,
				)
				continue
			}
			summaries = append(summaries, summary)
		}
	}

	logger.Info("cluster templates listed",
		"count", len(summaries),
		"namespace_count", len(namespaces),
	)

	return summaries, nil
}

// templateToSummary extracts key fields from a ClusterTemplate CR into a ClusterTemplateSummary.
func (m *Manager) templateToSummary(obj *unstructured.Unstructured) (ClusterTemplateSummary, error) {
	summary := ClusterTemplateSummary{
		Name:      obj.GetName(),
		Namespace: obj.GetNamespace(),
		Labels:    obj.GetLabels(),
		CreatedAt: obj.GetCreationTimestamp().Time,
	}

	// Extract description from annotations or spec
	annotations := obj.GetAnnotations()
	if desc, ok := annotations["k0rdent.mirantis.com/description"]; ok {
		summary.Description = desc
	} else if desc, found, err := unstructured.NestedString(obj.Object, "spec", "description"); err == nil && found {
		summary.Description = desc
	}

	// Extract provider from labels
	labels := obj.GetLabels()
	if provider, ok := labels["k0rdent.mirantis.com/provider"]; ok {
		summary.Provider = provider
	}
	if cloud, ok := labels["k0rdent.mirantis.com/cloud"]; ok {
		summary.Cloud = cloud
	}

	// Extract version from spec.version, annotations, or name pattern
	if version, found, err := unstructured.NestedString(obj.Object, "spec", "version"); err == nil && found {
		summary.Version = version
	} else if version, ok := annotations["k0rdent.mirantis.com/version"]; ok {
		summary.Version = version
	} else {
		// Try to extract from name if pattern matches
		summary.Version = extractVersionFromName(obj.GetName())
	}

	return summary, nil
}

// extractVersionFromName attempts to extract version suffix from template name.
// Common patterns: "aws-standalone-cp-1-0-15" -> "1.0.15"
func extractVersionFromName(name string) string {
	// Look for version pattern at the end (digits separated by dashes)
	// This is a heuristic - templates should use annotations for reliability
	var versionParts []rune
	dashCount := 0
	inVersion := false

	// Scan from right to left
	for i := len(name) - 1; i >= 0; i-- {
		c := rune(name[i])
		if c >= '0' && c <= '9' {
			versionParts = append([]rune{c}, versionParts...)
			inVersion = true
		} else if c == '-' && inVersion {
			dashCount++
			versionParts = append([]rune{c}, versionParts...)
		} else if inVersion && dashCount >= 2 {
			// Found end of version pattern
			break
		} else {
			// Reset if we haven't found a valid pattern yet
			versionParts = nil
			dashCount = 0
			inVersion = false
		}
	}

	if dashCount >= 2 && len(versionParts) > 0 {
		// Remove leading dash if present
		version := string(versionParts)
		if len(version) > 0 && version[0] == '-' {
			version = version[1:]
		}
		return dashesToDots(version)
	}
	return ""
}

// dashesToDots converts "1-0-15" to "1.0.15"
func dashesToDots(s string) string {
	b := make([]byte, len(s))
	for i := 0; i < len(s); i++ {
		if s[i] == '-' {
			b[i] = '.'
		} else {
			b[i] = s[i]
		}
	}
	return string(b)
}
