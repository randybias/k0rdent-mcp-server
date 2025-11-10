package helm

import (
	"fmt"
	"strings"

	"sigs.k8s.io/yaml"
)

// BuildKGSTValues creates the values map for installing a k0rdent catalog template via kgst
func (c *Client) BuildKGSTValues(template, version, namespace string) map[string]interface{} {
	values := map[string]interface{}{
		"chart": fmt.Sprintf("%s:%s", template, version),
		"repo": map[string]interface{}{
			"name": "k0rdent-catalog",
			"spec": map[string]interface{}{
				"url":  "oci://ghcr.io/k0rdent/catalog/charts",
				"type": "oci",
			},
		},
		"namespace":           namespace,
		"k0rdentApiVersion":   "v1beta1",
		"skipVerifyJob":       false,
	}

	// Log the values at debug level (without sensitive data)
	if c.logger != nil {
		valuesYAML, _ := yaml.Marshal(values)
		c.logger.Debug("kgst values constructed", 
			"template", template, 
			"version", version,
			"namespace", namespace,
			"values", string(valuesYAML))
	}

	return values
}

// ValidateKGSTValues validates the values structure for kgst installation
func (c *Client) ValidateKGSTValues(values map[string]interface{}) error {
	if values == nil {
		return fmt.Errorf("values cannot be nil")
	}

	// Check required fields
	if chart, ok := values["chart"]; !ok || chart == nil {
		return fmt.Errorf("missing required field: chart")
	}

	if repo, ok := values["repo"]; !ok || repo == nil {
		return fmt.Errorf("missing required field: repo")
	}

	repoMap, ok := values["repo"].(map[string]interface{})
	if !ok {
		return fmt.Errorf("repo must be a map")
	}

	specValue, ok := repoMap["spec"]
	if !ok || specValue == nil {
		return fmt.Errorf("missing required field: repo.spec")
	}

	specMap, ok := specValue.(map[string]interface{})
	if !ok {
		return fmt.Errorf("repo.spec must be a map")
	}

	if url, ok := specMap["url"]; !ok || url == nil {
		return fmt.Errorf("missing required field: repo.spec.url")
	}

	if repoType, ok := specMap["type"]; !ok || repoType == nil {
		return fmt.Errorf("missing required field: repo.spec.type")
	}

	// Validate chart format
	chartStr, ok := values["chart"].(string)
	if !ok || chartStr == "" {
		return fmt.Errorf("chart must be a non-empty string")
	}

	// Chart should be in format "name:version"
	if len(strings.Split(chartStr, ":")) != 2 {
		return fmt.Errorf("chart must be in format 'name:version', got: %s", chartStr)
	}

	return nil
}
