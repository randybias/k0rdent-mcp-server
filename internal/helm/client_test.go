package helm

import (
	"log/slog"
	"testing"
)

func TestNewClient(t *testing.T) {
	logger := slog.Default()
	
	// Test basic client creation
	client, err := NewClient(nil, "test-namespace", logger)
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}
	if client == nil {
		t.Fatal("Client is nil")
	}
	if client.namespace != "test-namespace" {
		t.Errorf("Expected namespace 'test-namespace', got '%s'", client.namespace)
	}
	if client.kgstVersion != DefaultKGSTVersion {
		t.Errorf("Expected kgst version '%s', got '%s'", DefaultKGSTVersion, client.kgstVersion)
	}
	
	// Test with custom version
	customVersion := "2.1.0"
	client2, err := NewClientWithVersion(nil, "test-namespace-2", logger, customVersion)
	if err != nil {
		t.Fatalf("Failed to create client with version: %v", err)
	}
	if client2.kgstVersion != customVersion {
		t.Errorf("Expected kgst version '%s', got '%s'", customVersion, client2.kgstVersion)
	}
}

func TestBuildKGSTValues(t *testing.T) {
	logger := slog.Default()
	client, err := NewClient(nil, "test-namespace", logger)
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}
	
	values := client.BuildKGSTValues("test-template", "1.2.3", "test-namespace")
	
	// Test basic structure
	chart, ok := values["chart"].(string)
	if !ok {
		t.Error("chart field is not a string")
	}
	if chart != "test-template:1.2.3" {
		t.Errorf("Expected chart 'test-template:1.2.3', got '%s'", chart)
	}
	
	repo, ok := values["repo"].(map[string]interface{})
	if !ok {
		t.Error("repo field is not a map")
	}
	
	name, ok := repo["name"].(string)
	if !ok {
		t.Error("repo.name is not a string")
	}
	if name != "k0rdent-catalog" {
		t.Errorf("Expected repo.name 'k0rdent-catalog', got '%s'", name)
	}
	
	spec, ok := repo["spec"].(map[string]interface{})
	if !ok {
		t.Error("repo.spec is not a map")
	}
	
	url, ok := spec["url"].(string)
	if !ok {
		t.Error("repo.spec.url is not a string")
	}
	if url != "oci://ghcr.io/k0rdent/catalog/charts" {
		t.Errorf("Expected repo.spec.url 'oci://ghcr.io/k0rdent/catalog/charts', got '%s'", url)
	}
	
	repoType, ok := spec["type"].(string)
	if !ok {
		t.Error("repo.spec.type is not a string")
	}
	if repoType != "oci" {
		t.Errorf("Expected repo.spec.type 'oci', got '%s'", repoType)
	}
	
	namespace, ok := values["namespace"].(string)
	if !ok {
		t.Error("namespace field is not a string")
	}
	if namespace != "test-namespace" {
		t.Errorf("Expected namespace 'test-namespace', got '%s'", namespace)
	}
}

func TestValidateKGSTValues(t *testing.T) {
	logger := slog.Default()
	client, err := NewClient(nil, "test-namespace", logger)
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}
	
	// Valid values should pass
	validValues := map[string]interface{}{
		"chart": "test-template:1.2.3",
		"repo": map[string]interface{}{
			"spec": map[string]interface{}{
				"url":  "oci://ghcr.io/k0rdent/catalog/charts",
				"type": "oci",
			},
		},
		"namespace": "test-namespace",
	}
	
	err = client.ValidateKGSTValues(validValues)
	if err != nil {
		t.Errorf("Valid values should not fail: %v", err)
	}
	
	// Test missing chart
	invalidValues1 := map[string]interface{}{
		"repo": map[string]interface{}{
			"spec": map[string]interface{}{
				"url":  "oci://ghcr.io/k0rdent/catalog/charts",
				"type": "oci",
			},
		},
	}
	
	err = client.ValidateKGSTValues(invalidValues1)
	if err == nil {
		t.Error("Missing chart should fail validation")
	}
	
	// Test invalid chart format
	invalidValues2 := map[string]interface{}{
		"chart": "invalid-format",
		"repo": map[string]interface{}{
			"spec": map[string]interface{}{
				"url":  "oci://ghcr.io/k0rdent/catalog/charts",
				"type": "oci",
			},
		},
	}
	
	err = client.ValidateKGSTValues(invalidValues2)
	if err == nil {
		t.Error("Invalid chart format should fail validation")
	}
	
	// Test missing repo.spec.url
	invalidValues3 := map[string]interface{}{
		"chart": "test-template:1.2.3",
		"repo": map[string]interface{}{
			"spec": map[string]interface{}{
				"type": "oci",
			},
		},
	}
	
	err = client.ValidateKGSTValues(invalidValues3)
	if err == nil {
		t.Error("Missing repo.spec.url should fail validation")
	}
	
	// Test nil values
	err = client.ValidateKGSTValues(nil)
	if err == nil {
		t.Error("Nil values should fail validation")
	}
}

func TestLoadChart(t *testing.T) {
	logger := slog.Default()
	client, err := NewClient(nil, "test-namespace", logger)
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}
	
	// Test chart URL construction
	chartRef, err := client.LoadChart(nil, "oci://example.com/chart", "1.0.0")
	if err != nil {
		t.Fatalf("Failed to load chart: %v", err)
	}
	if chartRef != "oci://example.com/chart:1.0.0" {
		t.Errorf("Expected 'oci://example.com/chart:1.0.0', got '%s'", chartRef)
	}
	
	// Test empty URL
	_, err = client.LoadChart(nil, "", "1.0.0")
	if err == nil {
		t.Error("Empty chart URL should fail")
	}
	
	// Test empty version (should use default)
	chartRef, err = client.LoadChart(nil, "oci://example.com/chart", "")
	if err != nil {
		t.Fatalf("Failed to load chart with default version: %v", err)
	}
	expectedDefault := "oci://example.com/chart:" + DefaultKGSTVersion
	if chartRef != expectedDefault {
		t.Errorf("Expected '%s', got '%s'", expectedDefault, chartRef)
	}
}

func TestLoadKGSTChart(t *testing.T) {
	logger := slog.Default()
	client, err := NewClient(nil, "test-namespace", logger)
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}
	
	// Test kgst chart loading
	chartRef, err := client.LoadKGSTChart(nil, "2.1.0")
	if err != nil {
		t.Fatalf("Failed to load kgst chart: %v", err)
	}
	expected := KGSTChartURL + ":2.1.0"
	if chartRef != expected {
		t.Errorf("Expected '%s', got '%s'", expected, chartRef)
	}
	
	// Test default version
	chartRef, err = client.LoadKGSTChart(nil, "")
	if err != nil {
		t.Fatalf("Failed to load kgst chart with default version: %v", err)
	}
	expectedDefault := KGSTChartURL + ":" + DefaultKGSTVersion
	if chartRef != expectedDefault {
		t.Errorf("Expected '%s', got '%s'", expectedDefault, chartRef)
	}
}
