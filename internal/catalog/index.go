package catalog

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// appMetadata represents the structure of apps/<slug>/data.yaml
type appMetadata struct {
	Title            string      `yaml:"title"`
	Summary          string      `yaml:"summary"`
	Tags             []string    `yaml:"tags"`
	ValidatedAWS     interface{} `yaml:"validated_aws"`
	ValidatedVsphere interface{} `yaml:"validated_vsphere"`
	ValidatedAzure   interface{} `yaml:"validated_azure"`
	ValidatedGCP     interface{} `yaml:"validated_gcp"`
	Charts           []struct {
		Name     string   `yaml:"name"`
		Versions []string `yaml:"versions"`
	} `yaml:"charts"`
}

// parseBoolField converts flexible validation field formats to bool.
// Accepts: true/false (bool), "y"/"yes" (string) → true, "-"/"n"/"no" (string) → false
func parseBoolField(val interface{}) bool {
	switch v := val.(type) {
	case bool:
		return v
	case string:
		switch strings.ToLower(strings.TrimSpace(v)) {
		case "y", "yes", "true":
			return true
		default:
			return false
		}
	default:
		return false
	}
}

// chartEntry represents a single entry in charts/st-charts.yaml
type chartEntry struct {
	Name       string `yaml:"name"`
	DepName    string `yaml:"dep_name"` // Dependency name in the catalog
	Version    string `yaml:"version"`
	Repository string `yaml:"repository"`
}

// stChartsFile represents the structure of st-charts.yaml
type stChartsFile struct {
	Charts []chartEntry `yaml:"st-charts"`
}

// buildDatabaseIndex walks the apps/ directory within the extracted catalog and
// populates the SQLite database with apps and their ServiceTemplate versions.
func buildDatabaseIndex(db *DB, extractDir string) error {
	appsDir := filepath.Join(extractDir, "apps")
	entries, err := os.ReadDir(appsDir)
	if err != nil {
		return fmt.Errorf("read apps directory: %w", err)
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		slug := entry.Name()
		appDir := filepath.Join(appsDir, slug)

		// Parse metadata - if it fails, skip this app rather than failing entire catalog
		metadata, err := parseDataYAML(appDir)
		if err != nil {
			// Apps with bad/missing metadata are simply not included
			continue
		}

		// For each chart/version, try to locate manifest
		templates := []ServiceTemplateRow{}

		// Prefer data.yaml charts section if available (new format)
		if len(metadata.Charts) > 0 {
			for _, chart := range metadata.Charts {
				for _, version := range chart.Versions {
					stPath, hrPath, err := locateManifests(appDir, chart.Name, version)
					if err != nil {
						// Skip incomplete versions
						continue
					}

					// Make paths relative to extractDir
					relStPath, err := filepath.Rel(extractDir, stPath)
					if err != nil {
						continue
					}

					var relHrPath string
					if hrPath != "" {
						relHrPath, err = filepath.Rel(extractDir, hrPath)
						if err != nil {
							continue
						}
					}

					templates = append(templates, ServiceTemplateRow{
						AppSlug:             slug,
						ChartName:           chart.Name,
						Version:             version,
						ServiceTemplatePath: relStPath,
						HelmRepositoryPath:  relHrPath,
					})
				}
			}
		} else {
			// Fall back to st-charts.yaml (legacy format for backward compatibility)
			versions, err := parseServiceTemplates(appDir, extractDir)
			if err != nil {
				// This can happen for apps still being developed or non-ServiceTemplate apps
				continue
			}

			for _, v := range versions {
				templates = append(templates, ServiceTemplateRow{
					AppSlug:             slug,
					ChartName:           v.Name,
					Version:             v.Version,
					ServiceTemplatePath: v.ServiceTemplatePath,
					HelmRepositoryPath:  v.HelmRepositoryPath,
				})
			}
		}

		// Only insert app if it has at least one complete template
		if len(templates) == 0 {
			continue
		}

		// Extract validated platforms
		platforms := []string{}
		if parseBoolField(metadata.ValidatedAWS) {
			platforms = append(platforms, "aws")
		}
		if parseBoolField(metadata.ValidatedVsphere) {
			platforms = append(platforms, "vsphere")
		}
		if parseBoolField(metadata.ValidatedAzure) {
			platforms = append(platforms, "azure")
		}
		if parseBoolField(metadata.ValidatedGCP) {
			platforms = append(platforms, "gcp")
		}

		// Insert into database
		appRow := AppRow{
			Slug:               slug,
			Title:              metadata.Title,
			Summary:            metadata.Summary,
			Tags:               metadata.Tags,
			ValidatedPlatforms: platforms,
		}

		if err := db.UpsertApp(appRow); err != nil {
			return fmt.Errorf("insert app %s: %w", slug, err)
		}

		for _, tmpl := range templates {
			if err := db.UpsertServiceTemplate(tmpl); err != nil {
				return fmt.Errorf("insert template %s/%s/%s: %w", slug, tmpl.ChartName, tmpl.Version, err)
			}
		}
	}

	return nil
}

// buildIndex walks the apps/ directory within the extracted catalog and constructs
// an in-memory index mapping app slugs to CatalogEntry structures.
// DEPRECATED: This function is kept for backward compatibility but will be removed
// once all code is migrated to use buildDatabaseIndex.
func buildIndex(extractDir string) (map[string]CatalogEntry, error) {
	appsDir := filepath.Join(extractDir, "apps")
	entries, err := os.ReadDir(appsDir)
	if err != nil {
		return nil, fmt.Errorf("read apps directory: %w", err)
	}

	index := make(map[string]CatalogEntry)
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		slug := entry.Name()
		appDir := filepath.Join(appsDir, slug)

		// Parse metadata - if it fails, skip this app rather than failing entire catalog
		metadata, err := parseAppMetadata(appDir)
		if err != nil {
			// Log but don't fail - catalog is under active development
			// Apps with bad/missing metadata are simply not included
			continue
		}

		// Parse ServiceTemplate versions - if it fails or empty, skip this app
		versions, err := parseServiceTemplates(appDir, extractDir)
		if err != nil {
			// This can happen for apps still being developed or non-ServiceTemplate apps
			continue
		}

		// Skip apps with no ServiceTemplates (e.g., ClusterDeployment templates)
		if len(versions) == 0 {
			continue
		}

		index[slug] = CatalogEntry{
			Slug:               slug,
			Title:              metadata.Title,
			Summary:            metadata.Summary,
			Tags:               metadata.Tags,
			ValidatedPlatforms: metadata.ValidatedPlatforms,
			Versions:           versions,
		}
	}

	return index, nil
}

// parseDataYAML reads apps/<slug>/data.yaml and returns the full appMetadata structure.
func parseDataYAML(appDir string) (*appMetadata, error) {
	dataPath := filepath.Join(appDir, "data.yaml")
	data, err := os.ReadFile(dataPath)
	if err != nil {
		return nil, fmt.Errorf("read data.yaml: %w", err)
	}

	var meta appMetadata
	if err := yaml.Unmarshal(data, &meta); err != nil {
		return nil, fmt.Errorf("unmarshal data.yaml: %w", err)
	}

	return &meta, nil
}

// parseAppMetadata reads apps/<slug>/data.yaml and extracts application metadata.
func parseAppMetadata(appDir string) (struct {
	Title              string
	Summary            string
	Tags               []string
	ValidatedPlatforms []string
}, error) {
	result := struct {
		Title              string
		Summary            string
		Tags               []string
		ValidatedPlatforms []string
	}{}

	meta, err := parseDataYAML(appDir)
	if err != nil {
		return result, err
	}

	result.Title = meta.Title
	result.Summary = meta.Summary
	result.Tags = meta.Tags

	// Collect validated platforms (handle both bool and string formats)
	platforms := []string{}
	if parseBoolField(meta.ValidatedAWS) {
		platforms = append(platforms, "aws")
	}
	if parseBoolField(meta.ValidatedVsphere) {
		platforms = append(platforms, "vsphere")
	}
	if parseBoolField(meta.ValidatedAzure) {
		platforms = append(platforms, "azure")
	}
	if parseBoolField(meta.ValidatedGCP) {
		platforms = append(platforms, "gcp")
	}
	result.ValidatedPlatforms = platforms

	return result, nil
}

// parseServiceTemplates reads charts/st-charts.yaml and locates manifest files
// for each ServiceTemplate version.
func parseServiceTemplates(appDir, extractDir string) ([]ServiceTemplateVersion, error) {
	chartsFile := filepath.Join(appDir, "charts", "st-charts.yaml")
	data, err := os.ReadFile(chartsFile)
	if err != nil {
		// If the charts directory or st-charts.yaml doesn't exist, this app has no ServiceTemplates
		// (e.g., it might be a ClusterDeployment template instead). Return empty list.
		if os.IsNotExist(err) {
			return []ServiceTemplateVersion{}, nil
		}
		return nil, fmt.Errorf("read st-charts.yaml: %w", err)
	}

	var file stChartsFile
	if err := yaml.Unmarshal(data, &file); err != nil {
		return nil, fmt.Errorf("unmarshal st-charts.yaml: %w", err)
	}

	charts := file.Charts

	versions := make([]ServiceTemplateVersion, 0, len(charts))
	for _, chart := range charts {
		// Locate manifest paths for this chart version
		stPath, hrPath, err := locateManifests(appDir, chart.Name, chart.Version)
		if err != nil {
			// Some apps have st-charts.yaml entries but no actual ServiceTemplate manifests yet.
			// Skip these entries rather than failing the entire app.
			// This is acceptable since the catalog is under active development.
			continue
		}

		// Make paths relative to extractDir
		relStPath, err := filepath.Rel(extractDir, stPath)
		if err != nil {
			return nil, fmt.Errorf("make service template path relative: %w", err)
		}
		var relHrPath string
		if hrPath != "" {
			relHrPath, err = filepath.Rel(extractDir, hrPath)
			if err != nil {
				return nil, fmt.Errorf("make helm repository path relative: %w", err)
			}
		}

		versions = append(versions, ServiceTemplateVersion{
			Name:                chart.Name,
			Version:             chart.Version,
			Repository:          chart.Repository,
			ServiceTemplatePath: relStPath,
			HelmRepositoryPath:  relHrPath,
		})
	}

	return versions, nil
}

// locateManifests finds the ServiceTemplate and optional HelmRepository YAML files
// for a given chart name and version. Returns absolute paths within the extracted archive.
func locateManifests(appDir, chartName, version string) (serviceTemplatePath, helmRepositoryPath string, err error) {
	// The catalog has multiple directory naming patterns. Try them in order:
	// 1. charts/<name>-service-template-<version>/templates/  (most common)
	// 2. charts/<name>-<version>/templates/                   (alternative pattern)

	patterns := []string{
		fmt.Sprintf("%s-service-template-%s", chartName, version),
		fmt.Sprintf("%s-%s", chartName, version),
	}

	var stPath string
	var templatesDir string

	for _, pattern := range patterns {
		chartDir := filepath.Join(appDir, "charts", pattern)
		templatesDir = filepath.Join(chartDir, "templates")
		stPath = filepath.Join(templatesDir, "service-template.yaml")

		if _, err := os.Stat(stPath); err == nil {
			// Found it!
			break
		}
	}

	// Verify we found a ServiceTemplate
	if _, err := os.Stat(stPath); err != nil {
		return "", "", fmt.Errorf("service-template.yaml not found in any expected location: %w", err)
	}

	// HelmRepository manifest is optional - check both the chart templates
	// and the k0rdent-catalog helper chart
	hrPath := filepath.Join(templatesDir, "helm-repository.yaml")
	if _, err := os.Stat(hrPath); err == nil {
		return stPath, hrPath, nil
	}

	// Check for k0rdent-catalog helper chart
	// The helper chart follows pattern: apps/k0rdent-utils/charts/k0rdent-catalog-<version>/templates/helm-repository.yaml
	// We need to find it relative to the apps directory
	appsDir := filepath.Dir(appDir)
	utilsDir := filepath.Join(appsDir, "k0rdent-utils", "charts")

	if entries, err := os.ReadDir(utilsDir); err == nil {
		for _, entry := range entries {
			if entry.IsDir() && strings.HasPrefix(entry.Name(), "k0rdent-catalog-") {
				helperHRPath := filepath.Join(utilsDir, entry.Name(), "templates", "helm-repository.yaml")
				if _, err := os.Stat(helperHRPath); err == nil {
					return stPath, helperHRPath, nil
				}
			}
		}
	}

	// HelmRepository not found - this is acceptable
	return stPath, "", nil
}
