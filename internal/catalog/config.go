package catalog

import (
	"os"
	"time"
)

const (
	// EnvArchiveURL overrides the default GitHub catalog tarball URL
	EnvArchiveURL = "CATALOG_ARCHIVE_URL"

	// EnvCacheDir overrides the default cache directory path
	EnvCacheDir = "CATALOG_CACHE_DIR"

	// EnvDownloadTimeout overrides the default HTTP request timeout
	EnvDownloadTimeout = "CATALOG_DOWNLOAD_TIMEOUT"

	// EnvCacheTTL overrides the default cache time-to-live
	EnvCacheTTL = "CATALOG_CACHE_TTL"

	// DefaultArchiveURL points to the main branch tarball of the k0rdent catalog
	DefaultArchiveURL = "https://github.com/k0rdent/catalog/archive/refs/heads/main.tar.gz"

	// DefaultCacheDir is the filesystem location for storing catalog data
	DefaultCacheDir = "/var/lib/k0rdent-mcp/catalog"

	// DefaultDownloadTimeout is the HTTP client timeout for archive downloads
	DefaultDownloadTimeout = 30 * time.Second

	// DefaultCacheTTL is how long cached catalog data remains valid
	DefaultCacheTTL = 6 * time.Hour
)

// LoadConfig reads configuration from environment variables and returns
// Options with defaults applied.
func LoadConfig() Options {
	opts := Options{
		ArchiveURL:      DefaultArchiveURL,
		CacheDir:        DefaultCacheDir,
		DownloadTimeout: DefaultDownloadTimeout,
		CacheTTL:        DefaultCacheTTL,
	}

	if url := os.Getenv(EnvArchiveURL); url != "" {
		opts.ArchiveURL = url
	}

	if dir := os.Getenv(EnvCacheDir); dir != "" {
		opts.CacheDir = dir
	}

	if timeout := os.Getenv(EnvDownloadTimeout); timeout != "" {
		if d, err := time.ParseDuration(timeout); err == nil {
			opts.DownloadTimeout = d
		}
	}

	if ttl := os.Getenv(EnvCacheTTL); ttl != "" {
		if d, err := time.ParseDuration(ttl); err == nil {
			opts.CacheTTL = d
		}
	}

	return opts
}
