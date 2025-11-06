package catalog

import (
	"log/slog"
	"net/http"
	"time"
)

// CatalogEntry represents a single application in the catalog with its metadata
// and available ServiceTemplate versions.
type CatalogEntry struct {
	// Slug is the unique identifier for the app (directory name under apps/)
	Slug string `json:"slug"`

	// Title is the human-readable name of the application
	Title string `json:"title"`

	// Summary provides a brief description of the application
	Summary string `json:"summary,omitempty"`

	// Tags are labels for categorizing the application
	Tags []string `json:"tags,omitempty"`

	// ValidatedPlatforms lists platform validation flags (aws, vsphere, etc)
	ValidatedPlatforms []string `json:"validated_platforms,omitempty"`

	// Versions lists all available ServiceTemplate versions for this app
	Versions []ServiceTemplateVersion `json:"versions"`
}

// ServiceTemplateVersion describes a specific version of a ServiceTemplate chart.
type ServiceTemplateVersion struct {
	// Name is the chart name (e.g., "postgresql")
	Name string `json:"name"`

	// Version is the semantic version (e.g., "1.2.3")
	Version string `json:"version"`

	// Repository is the OCI repository URL
	Repository string `json:"repository"`

	// ServiceTemplatePath is the relative path to service-template.yaml within the extracted archive
	ServiceTemplatePath string `json:"service_template_path"`

	// HelmRepositoryPath is the optional path to helm-repository.yaml
	HelmRepositoryPath string `json:"helm_repository_path,omitempty"`
}

// CacheMetadata tracks cache state for validation and refresh decisions.
type CacheMetadata struct {
	// SHA is the commit SHA or ETag from the downloaded archive
	SHA string `json:"sha"`

	// Timestamp records when the cache was last updated
	Timestamp time.Time `json:"timestamp"`

	// URL is the archive URL that was fetched
	URL string `json:"url"`
}

// Options configure the catalog Manager.
type Options struct {
	// HTTPClient is used for downloading the catalog archive (optional, defaults to http.DefaultClient with timeout)
	HTTPClient *http.Client

	// CacheDir is the directory where catalog archives are stored (required)
	CacheDir string

	// CacheTTL determines how long cached data remains valid before refresh
	CacheTTL time.Duration

	// ArchiveURL is the URL to download the catalog tarball
	ArchiveURL string

	// DownloadTimeout is the HTTP request timeout for archive downloads
	DownloadTimeout time.Duration

	// Logger is used for structured logging (optional, defaults to slog.Default())
	Logger *slog.Logger
}
