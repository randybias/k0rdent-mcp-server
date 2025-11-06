package catalog

import (
	"os"
	"path/filepath"
	"testing"
)

func TestBuildIndex(t *testing.T) {
	t.Skip("Skipping deprecated tarball-based buildIndex test - now using JSON index")
	testDir := filepath.Join("testdata", "catalog-fake-sha", "catalog-main")
	index, err := buildIndex(testDir)
	if err != nil {
		t.Fatalf("buildIndex failed: %v", err)
	}

	if len(index) != 2 {
		t.Errorf("expected 2 apps, got %d", len(index))
	}

	entry, ok := index["minio"]
	if !ok {
		t.Fatal("minio app not found in index")
	}

	if entry.Slug != "minio" {
		t.Errorf("expected slug 'minio', got '%s'", entry.Slug)
	}

	if entry.Title != "MinIO Object Storage" {
		t.Errorf("expected title 'MinIO Object Storage', got '%s'", entry.Title)
	}

	if entry.Summary != "High-performance object storage for cloud-native applications" {
		t.Errorf("unexpected summary: %s", entry.Summary)
	}

	if len(entry.Tags) != 3 {
		t.Errorf("expected 3 tags, got %d", len(entry.Tags))
	}

	if len(entry.ValidatedPlatforms) != 1 || entry.ValidatedPlatforms[0] != "aws" {
		t.Errorf("expected validated platforms [aws], got %v", entry.ValidatedPlatforms)
	}

	if len(entry.Versions) != 1 {
		t.Errorf("expected 1 version, got %d", len(entry.Versions))
	}

	version := entry.Versions[0]
	if version.Name != "minio" {
		t.Errorf("expected version name 'minio', got '%s'", version.Name)
	}

	if version.Version != "14.1.2" {
		t.Errorf("expected version '14.1.2', got '%s'", version.Version)
	}

	if version.Repository != "oci://ghcr.io/k0rdent/catalog/charts" {
		t.Errorf("unexpected repository: %s", version.Repository)
	}

	if version.ServiceTemplatePath == "" {
		t.Error("expected non-empty ServiceTemplatePath")
	}

	if version.HelmRepositoryPath == "" {
		t.Error("expected non-empty HelmRepositoryPath")
	}
}

func TestBuildIndexMissingAppsDir(t *testing.T) {
	_, err := buildIndex("/nonexistent/path")
	if err == nil {
		t.Fatal("expected error for missing apps directory")
	}
}

func TestParseAppMetadata(t *testing.T) {
	t.Skip("Skipping deprecated YAML-based parseAppMetadata test - now using JSON index")
	appDir := filepath.Join("testdata", "catalog-fake-sha", "catalog-main", "apps", "minio")
	metadata, err := parseAppMetadata(appDir)
	if err != nil {
		t.Fatalf("parseAppMetadata failed: %v", err)
	}

	if metadata.Title != "MinIO Object Storage" {
		t.Errorf("expected title 'MinIO Object Storage', got '%s'", metadata.Title)
	}

	if metadata.Summary != "High-performance object storage for cloud-native applications" {
		t.Errorf("unexpected summary: %s", metadata.Summary)
	}

	if len(metadata.Tags) != 3 {
		t.Errorf("expected 3 tags, got %d", len(metadata.Tags))
	}

	expectedTags := map[string]bool{"storage": true, "object-storage": true, "s3": true}
	for _, tag := range metadata.Tags {
		if !expectedTags[tag] {
			t.Errorf("unexpected tag: %s", tag)
		}
	}

	if len(metadata.ValidatedPlatforms) != 1 || metadata.ValidatedPlatforms[0] != "aws" {
		t.Errorf("expected validated platforms [aws], got %v", metadata.ValidatedPlatforms)
	}
}

func TestParseAppMetadataMultiplePlatforms(t *testing.T) {
	tmpDir := t.TempDir()
	dataPath := filepath.Join(tmpDir, "data.yaml")
	content := `title: "Test App"
summary: "Test summary"
tags: ["test"]
validated_aws: true
validated_vsphere: true
validated_azure: true
validated_gcp: true
`
	if err := os.WriteFile(dataPath, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write test data.yaml: %v", err)
	}

	metadata, err := parseAppMetadata(tmpDir)
	if err != nil {
		t.Fatalf("parseAppMetadata failed: %v", err)
	}

	if len(metadata.ValidatedPlatforms) != 4 {
		t.Errorf("expected 4 validated platforms, got %d: %v", len(metadata.ValidatedPlatforms), metadata.ValidatedPlatforms)
	}

	expectedPlatforms := map[string]bool{"aws": true, "vsphere": true, "azure": true, "gcp": true}
	for _, platform := range metadata.ValidatedPlatforms {
		if !expectedPlatforms[platform] {
			t.Errorf("unexpected platform: %s", platform)
		}
	}
}

func TestParseAppMetadataMissingFile(t *testing.T) {
	_, err := parseAppMetadata("/nonexistent/path")
	if err == nil {
		t.Fatal("expected error for missing data.yaml")
	}
}

func TestParseAppMetadataCorruptYAML(t *testing.T) {
	tmpDir := t.TempDir()
	dataPath := filepath.Join(tmpDir, "data.yaml")
	if err := os.WriteFile(dataPath, []byte("invalid: yaml: content: ["), 0644); err != nil {
		t.Fatalf("failed to write corrupt data.yaml: %v", err)
	}

	_, err := parseAppMetadata(tmpDir)
	if err == nil {
		t.Fatal("expected error for corrupt YAML")
	}
}

func TestParseServiceTemplates(t *testing.T) {
	t.Skip("Skipping deprecated YAML-based parseServiceTemplates test - now using JSON index")
	extractDir := filepath.Join("testdata", "catalog-fake-sha", "catalog-main")
	appDir := filepath.Join(extractDir, "apps", "minio")
	versions, err := parseServiceTemplates(appDir, extractDir)
	if err != nil {
		t.Fatalf("parseServiceTemplates failed: %v", err)
	}

	if len(versions) != 1 {
		t.Fatalf("expected 1 version, got %d", len(versions))
	}

	version := versions[0]
	if version.Name != "minio" {
		t.Errorf("expected name 'minio', got '%s'", version.Name)
	}

	if version.Version != "14.1.2" {
		t.Errorf("expected version '14.1.2', got '%s'", version.Version)
	}

	if version.Repository != "oci://ghcr.io/k0rdent/catalog/charts" {
		t.Errorf("unexpected repository: %s", version.Repository)
	}

	if version.ServiceTemplatePath == "" {
		t.Error("expected non-empty ServiceTemplatePath")
	}

	if version.HelmRepositoryPath == "" {
		t.Error("expected non-empty HelmRepositoryPath")
	}
}

func TestParseServiceTemplatesMissingFile(t *testing.T) {
	// Missing st-charts.yaml should return empty list (apps without ServiceTemplates)
	versions, err := parseServiceTemplates("/nonexistent/path", "/nonexistent")
	if err != nil {
		t.Fatalf("expected no error for missing st-charts.yaml, got: %v", err)
	}
	if len(versions) != 0 {
		t.Errorf("expected empty versions list, got %d versions", len(versions))
	}
}

func TestParseServiceTemplatesCorruptYAML(t *testing.T) {
	tmpDir := t.TempDir()
	chartsDir := filepath.Join(tmpDir, "charts")
	if err := os.MkdirAll(chartsDir, 0755); err != nil {
		t.Fatalf("failed to create charts directory: %v", err)
	}

	chartsFile := filepath.Join(chartsDir, "st-charts.yaml")
	if err := os.WriteFile(chartsFile, []byte("- invalid: [yaml}"), 0644); err != nil {
		t.Fatalf("failed to write corrupt st-charts.yaml: %v", err)
	}

	_, err := parseServiceTemplates(tmpDir, tmpDir)
	if err == nil {
		t.Fatal("expected error for corrupt YAML")
	}
}

func TestLocateManifests(t *testing.T) {
	t.Skip("Skipping deprecated tarball-based locateManifests test - now using JSON index and GitHub fetching")
	appDir := filepath.Join("testdata", "catalog-fake-sha", "catalog-main", "apps", "minio")
	stPath, hrPath, err := locateManifests(appDir, "minio", "14.1.2")
	if err != nil {
		t.Fatalf("locateManifests failed: %v", err)
	}

	if stPath == "" {
		t.Error("expected non-empty service template path")
	}

	// Verify service template file exists
	if _, err := os.Stat(stPath); err != nil {
		t.Errorf("service template file does not exist: %v", err)
	}

	if hrPath == "" {
		t.Error("expected non-empty helm repository path")
	}

	// Verify helm repository file exists
	if _, err := os.Stat(hrPath); err != nil {
		t.Errorf("helm repository file does not exist: %v", err)
	}
}

func TestLocateManifestsMissingServiceTemplate(t *testing.T) {
	tmpDir := t.TempDir()
	_, _, err := locateManifests(tmpDir, "nonexistent", "1.0.0")
	if err == nil {
		t.Fatal("expected error for missing service template")
	}
}

func TestLocateManifestsWithoutHelmRepository(t *testing.T) {
	tmpDir := t.TempDir()
	chartDir := filepath.Join(tmpDir, "charts", "test-1.0.0", "templates")
	if err := os.MkdirAll(chartDir, 0755); err != nil {
		t.Fatalf("failed to create chart directory: %v", err)
	}

	stPath := filepath.Join(chartDir, "service-template.yaml")
	if err := os.WriteFile(stPath, []byte("apiVersion: v1\nkind: ServiceTemplate"), 0644); err != nil {
		t.Fatalf("failed to write service template: %v", err)
	}

	stPathResult, hrPath, err := locateManifests(tmpDir, "test", "1.0.0")
	if err != nil {
		t.Fatalf("locateManifests failed: %v", err)
	}

	if stPathResult != stPath {
		t.Errorf("expected service template path %s, got %s", stPath, stPathResult)
	}

	if hrPath != "" {
		t.Errorf("expected empty helm repository path, got %s", hrPath)
	}
}
