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
	// CredentialsGVR is the GroupVersionResource for Credential CRs
	CredentialsGVR = schema.GroupVersionResource{
		Group:    "k0rdent.mirantis.com",
		Version:  "v1beta1",
		Resource: "credentials",
	}
)

// ListCredentials retrieves Credential resources from the specified namespaces.
// Returns summaries with key metadata including provider, readiness, and labels.
func (m *Manager) ListCredentials(ctx context.Context, namespaces []string) ([]CredentialSummary, error) {
	logger := logging.WithContext(ctx, m.logger)
	logger.Debug("listing credentials", "namespace_count", len(namespaces))

	if len(namespaces) == 0 {
		logger.Warn("no namespaces provided for credential listing")
		return []CredentialSummary{}, nil
	}

	var summaries []CredentialSummary

	// Query each namespace
	for _, ns := range namespaces {
		logger.Debug("listing credentials in namespace", "namespace", ns)

		list, err := m.dynamicClient.Resource(CredentialsGVR).Namespace(ns).List(ctx, metav1.ListOptions{})
		if err != nil {
			logger.Error("failed to list credentials in namespace",
				"namespace", ns,
				"error", err,
			)
			return nil, fmt.Errorf("list credentials in namespace %s: %w", ns, err)
		}

		logger.Debug("found credentials in namespace",
			"namespace", ns,
			"count", len(list.Items),
		)

		// Convert each credential to summary
		for _, item := range list.Items {
			summary, err := m.credentialToSummary(&item)
			if err != nil {
				logger.Warn("failed to convert credential to summary",
					"namespace", ns,
					"name", item.GetName(),
					"error", err,
				)
				continue
			}
			summaries = append(summaries, summary)
		}
	}

	logger.Info("credentials listed",
		"count", len(summaries),
		"namespace_count", len(namespaces),
	)

	return summaries, nil
}

// credentialToSummary extracts key fields from a Credential CR into a CredentialSummary.
func (m *Manager) credentialToSummary(obj *unstructured.Unstructured) (CredentialSummary, error) {
	summary := CredentialSummary{
		Name:      obj.GetName(),
		Namespace: obj.GetNamespace(),
		Labels:    obj.GetLabels(),
		CreatedAt: obj.GetCreationTimestamp().Time,
	}

	// Extract provider from spec.identityRef.kind or labels
	if identityRef, found, err := unstructured.NestedMap(obj.Object, "spec", "identityRef"); err == nil && found {
		if kind, ok := identityRef["kind"].(string); ok {
			// Map kind to provider (e.g., "AWSClusterIdentity" -> "aws")
			summary.Provider = normalizeProviderKind(kind)
		}
	}

	// Fallback to label-based provider detection
	if summary.Provider == "" {
		if provider, ok := obj.GetLabels()["k0rdent.mirantis.com/provider"]; ok {
			summary.Provider = provider
		}
	}

	// Check readiness from status.conditions
	summary.Ready = IsResourceReady(obj)

	return summary, nil
}

// normalizeProviderKind converts Kubernetes kind names to provider identifiers.
// Examples: "AWSClusterIdentity" -> "aws", "AzureClusterIdentity" -> "azure"
func normalizeProviderKind(kind string) string {
	if len(kind) == 0 {
		return ""
	}

	// Common patterns
	switch kind {
	case "AWSClusterIdentity", "AWSClusterStaticIdentity", "AWSClusterRoleIdentity":
		return "aws"
	case "AzureClusterIdentity":
		return "azure"
	case "VSphereClusterIdentity":
		return "vsphere"
	case "GCPClusterIdentity":
		return "gcp"
	}

	// Generic fallback: lowercase the prefix before "Cluster"
	for i := 0; i < len(kind); i++ {
		if i+7 < len(kind) && kind[i:i+7] == "Cluster" {
			return toLower(kind[:i])
		}
	}

	return toLower(kind)
}

// toLower converts a string to lowercase (simplified version without unicode handling).
func toLower(s string) string {
	b := make([]byte, len(s))
	for i := 0; i < len(s); i++ {
		c := s[i]
		if 'A' <= c && c <= 'Z' {
			c += 'a' - 'A'
		}
		b[i] = c
	}
	return string(b)
}
