package clusters

import (
	"context"
	"fmt"

	"github.com/k0rdent/mcp-k0rdent-server/internal/logging"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// ResolveTargetNamespace determines which namespace to use for a cluster operation.
// It implements the auth mode-aware logic described in the design doc:
// - If explicit namespace is provided, validate against filter
// - If no namespace and filter is nil or matches kcm-system: default to kcm-system (DEV mode)
// - If no namespace and filter exists and doesn't match kcm-system: require explicit namespace (OIDC mode)
func (m *Manager) ResolveTargetNamespace(ctx context.Context, explicitNamespace string) (string, error) {
	logger := logging.WithContext(ctx, m.logger)

	// Case 1: Explicit namespace provided - validate against filter
	if explicitNamespace != "" {
		if m.namespaceFilter != nil && !m.namespaceFilter.MatchString(explicitNamespace) {
			logger.Warn("namespace not allowed by filter",
				"namespace", explicitNamespace,
				"filter", m.namespaceFilter.String(),
			)
			return "", fmt.Errorf("%w: %s", ErrNamespaceForbidden, explicitNamespace)
		}
		logger.Debug("resolved explicit namespace", "namespace", explicitNamespace)
		return explicitNamespace, nil
	}

	// Case 2: No explicit namespace - determine default behavior
	// DEV_ALLOW_ANY mode (no filter or matches kcm-system): default to global namespace
	if m.namespaceFilter == nil || m.namespaceFilter.MatchString(m.globalNamespace) {
		logger.Debug("defaulting to global namespace (DEV mode)",
			"namespace", m.globalNamespace,
		)
		return m.globalNamespace, nil
	}

	// OIDC_REQUIRED mode - require explicit namespace
	logger.Warn("namespace required in OIDC mode but not provided")
	return "", ErrNamespaceRequired
}

// GetAllowedNamespaces returns all namespaces that match the namespace filter.
// This is used for listing operations that need to query multiple namespaces.
// Returns global namespace plus all filter-matched namespaces.
func (m *Manager) GetAllowedNamespaces(ctx context.Context) ([]string, error) {
	logger := logging.WithContext(ctx, m.logger)
	logger.Debug("resolving allowed namespaces")

	// List all namespaces from the cluster
	nsGVR := schema.GroupVersionResource{
		Group:    "",
		Version:  "v1",
		Resource: "namespaces",
	}

	nsList, err := m.dynamicClient.Resource(nsGVR).List(ctx, metav1.ListOptions{})
	if err != nil {
		logger.Error("failed to list namespaces", "error", err)
		return nil, fmt.Errorf("list namespaces: %w", err)
	}

	// Collect allowed namespaces based on filter
	allowed := make([]string, 0)
	seenGlobal := false

	for _, ns := range nsList.Items {
		nsName := ns.GetName()

		// Always include global namespace
		if nsName == m.globalNamespace {
			seenGlobal = true
			allowed = append(allowed, nsName)
			continue
		}

		// If no filter, all namespaces are allowed
		if m.namespaceFilter == nil {
			allowed = append(allowed, nsName)
		} else if m.namespaceFilter.MatchString(nsName) {
			allowed = append(allowed, nsName)
		}
	}

	// Ensure global namespace is always included (even if not in list yet)
	if !seenGlobal {
		allowed = append([]string{m.globalNamespace}, allowed...)
	}

	if len(allowed) == 0 {
		logger.Warn("no allowed namespaces found")
		return nil, ErrNoAllowedNamespaces
	}

	logger.Debug("resolved allowed namespaces",
		"count", len(allowed),
		"namespaces", allowed,
	)

	return allowed, nil
}

// ResolveResourceNamespace determines which namespace a resource reference points to.
// Handles both "name" and "namespace/name" formats.
// Falls back to target namespace if no explicit namespace in reference.
func (m *Manager) ResolveResourceNamespace(ctx context.Context, reference, targetNamespace string) (namespace, name string, err error) {
	logger := logging.WithContext(ctx, m.logger)

	// Check if reference contains namespace (format: "namespace/name")
	if ns, n, ok := splitNamespaceAndName(reference); ok {
		// Validate against filter
		if m.namespaceFilter != nil && !m.namespaceFilter.MatchString(ns) {
			logger.Warn("resource namespace not allowed by filter",
				"reference", reference,
				"namespace", ns,
				"filter", m.namespaceFilter.String(),
			)
			return "", "", fmt.Errorf("%w: %s", ErrNamespaceForbidden, ns)
		}
		logger.Debug("resolved resource with explicit namespace",
			"reference", reference,
			"namespace", ns,
			"name", n,
		)
		return ns, n, nil
	}

	// No explicit namespace - use target namespace
	logger.Debug("resolved resource to target namespace",
		"reference", reference,
		"namespace", targetNamespace,
	)
	return targetNamespace, reference, nil
}

// splitNamespaceAndName splits "namespace/name" into components.
// Returns (namespace, name, true) if split succeeds, ("", "", false) otherwise.
func splitNamespaceAndName(reference string) (string, string, bool) {
	for i := 0; i < len(reference); i++ {
		if reference[i] == '/' {
			if i == 0 || i == len(reference)-1 {
				return "", "", false
			}
			return reference[:i], reference[i+1:], true
		}
	}
	return "", "", false
}
