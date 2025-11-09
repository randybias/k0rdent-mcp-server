package clusters

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/k0rdent/mcp-k0rdent-server/internal/logging"
)

// SelectLatestTemplate finds the latest stable template for the specified provider.
// It filters templates by provider prefix pattern (e.g., "aws-standalone-cp-") and
// returns the template name with the highest semantic version.
// Returns error if no matching templates exist in the namespace.
func (m *Manager) SelectLatestTemplate(ctx context.Context, provider string, namespace string) (string, error) {
	logger := logging.WithContext(ctx, m.logger)
	logger.Debug("selecting latest template",
		"provider", provider,
		"namespace", namespace,
	)

	// List all templates in the namespace
	templates, err := m.ListTemplates(ctx, []string{namespace})
	if err != nil {
		return "", fmt.Errorf("list templates: %w", err)
	}

	// Filter by provider prefix pattern (e.g., "aws-standalone-cp-")
	pattern := fmt.Sprintf("%s-standalone-cp-", provider)
	var matching []ClusterTemplateSummary
	for _, t := range templates {
		if strings.HasPrefix(t.Name, pattern) {
			matching = append(matching, t)
		}
	}

	if len(matching) == 0 {
		logger.Warn("no matching templates found",
			"provider", provider,
			"namespace", namespace,
			"pattern", pattern,
		)
		return "", fmt.Errorf("no templates found for provider %s in namespace %s", provider, namespace)
	}

	logger.Debug("found matching templates",
		"count", len(matching),
		"provider", provider,
	)

	// Sort by version (descending - highest version first)
	sort.Slice(matching, func(i, j int) bool {
		return compareVersions(matching[i].Version, matching[j].Version) > 0
	})

	latest := matching[0]
	logger.Info("selected latest template",
		"template", latest.Name,
		"version", latest.Version,
		"provider", provider,
		"namespace", namespace,
	)

	return latest.Name, nil
}

// compareVersions compares two semantic version strings.
// Returns:
//   - 1 if v1 > v2
//   - -1 if v1 < v2
//   - 0 if v1 == v2
//
// Version strings are expected to be in format "major.minor.patch" (e.g., "1.0.14").
// Invalid versions are treated as "0.0.0".
func compareVersions(v1, v2 string) int {
	parts1 := parseVersion(v1)
	parts2 := parseVersion(v2)

	// Compare major version
	if parts1[0] != parts2[0] {
		if parts1[0] > parts2[0] {
			return 1
		}
		return -1
	}

	// Compare minor version
	if parts1[1] != parts2[1] {
		if parts1[1] > parts2[1] {
			return 1
		}
		return -1
	}

	// Compare patch version
	if parts1[2] != parts2[2] {
		if parts1[2] > parts2[2] {
			return 1
		}
		return -1
	}

	return 0
}

// parseVersion parses a semantic version string into [major, minor, patch].
// Returns [0, 0, 0] for invalid or empty versions.
func parseVersion(version string) [3]int {
	var parts [3]int

	if version == "" {
		return parts
	}

	// Split by dots
	segments := strings.Split(version, ".")
	for i := 0; i < 3 && i < len(segments); i++ {
		// Parse as integer, default to 0 on error
		if num, err := strconv.Atoi(strings.TrimSpace(segments[i])); err == nil {
			parts[i] = num
		}
	}

	return parts
}
