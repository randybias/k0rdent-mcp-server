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

	// IndexTimestamp stores the metadata.generated timestamp from the JSON index
	// Used to determine if the catalog index has changed without re-downloading
	IndexTimestamp string `json:"index_timestamp,omitempty"`
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

// JSONIndex represents the structure of the JSON catalog index downloaded from the catalog repository.
type JSONIndex struct {
	// Metadata contains versioning and generation information about the index
	Metadata JSONMetadata `json:"metadata"`

	// Addons is the list of all applications/addons in the catalog
	Addons []JSONAddon `json:"addons"`
}

// JSONMetadata contains versioning and timestamp information for the JSON index.
type JSONMetadata struct {
	// Generated is the timestamp when the index was generated
	Generated string `json:"generated"`

	// Version is the schema version of the index format
	Version string `json:"version"`
}

// JSONAddon represents a single addon/application entry in the JSON catalog index.
type JSONAddon struct {
	// Name is the unique identifier for the addon (maps to CatalogEntry.Slug)
	Name string `json:"name"`

	// Description provides a brief description of the addon (maps to CatalogEntry.Summary)
	Description string `json:"description"`

	// LatestVersion is the most recent version available for this addon
	LatestVersion string `json:"latestVersion"`

	// Versions lists all available versions for this addon
	Versions []string `json:"versions"`

	// Charts lists the Helm charts associated with this addon
	Charts []JSONChart `json:"charts"`

	// Metadata contains additional metadata about the addon
	Metadata JSONAddonMetadata `json:"metadata"`
}

// JSONChart represents a Helm chart within an addon.
type JSONChart struct {
	// Name is the chart name (maps to ServiceTemplateVersion.Name)
	Name string `json:"name"`

	// Versions lists all available versions of this chart (maps to ServiceTemplateVersion.Version)
	Versions []string `json:"versions"`
}

// JSONAddonMetadata contains additional metadata fields for an addon.
type JSONAddonMetadata struct {
	// Tags are labels for categorizing the addon
	Tags []string `json:"tags"`

	// Owner identifies the team or organization maintaining this addon
	Owner string `json:"owner"`
}
