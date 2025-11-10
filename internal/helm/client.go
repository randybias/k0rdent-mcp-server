package helm

import (
	"fmt"
	"log/slog"
	"os"
)

// Client represents a Helm client for installing kgst charts
// This implementation uses the helm CLI instead of the Helm SDK to avoid compatibility issues
type Client struct {
	namespace    string
	logger       *slog.Logger
	kgstVersion  string
}

// NewClient creates a new Helm client configured for the given namespace
func NewClient(restConfig interface{}, namespace string, logger *slog.Logger) (*Client, error) {
	return NewClientWithVersion(restConfig, namespace, logger, "")
}

// NewClientWithVersion creates a new Helm client with a specific kgst version
func NewClientWithVersion(_ interface{}, namespace string, logger *slog.Logger, kgstVersion string) (*Client, error) {
	if namespace == "" {
		return nil, fmt.Errorf("namespace is required")
	}
	if logger == nil {
		logger = slog.Default()
	}

	// Default kgst version if not provided
	if kgstVersion == "" {
		kgstVersion = DefaultKGSTVersion
		// Check for environment variable override
		if envVersion := os.Getenv("KGST_CHART_VERSION"); envVersion != "" {
			kgstVersion = envVersion
		}
	}

	client := &Client{
		namespace:   namespace,
		logger:      logger,
		kgstVersion: kgstVersion,
	}

	logger.Debug("Helm client created (CLI implementation)", "namespace", namespace, "kgst_version", kgstVersion)
	return client, nil
}

// Close releases any resources held by the Helm client
func (c *Client) Close() {
	c.logger.Debug("Helm client closed (CLI implementation)")
}
