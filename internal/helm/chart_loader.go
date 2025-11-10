package helm

import (
	"context"
	"fmt"
)

const (
	// KGSTChartURL is the OCI URL for the kgst chart
	KGSTChartURL = "oci://ghcr.io/k0rdent/catalog/charts/kgst"
	// DefaultKGSTVersion is the default kgst chart version to use
	DefaultKGSTVersion = "2.0.0"
)

// LoadChart validates chart URL and version for CLI usage
func (c *Client) LoadChart(ctx context.Context, chartURL string, version string) (string, error) {
	if chartURL == "" {
		return "", fmt.Errorf("chart URL is required")
	}
	if version == "" {
		version = DefaultKGSTVersion
	}

	c.logger.Debug("validating chart", 
		"url", chartURL, 
		"version", version)

	// Construct the full OCI chart reference with version
	chartRef := chartURL + ":" + version

	c.logger.Debug("chart reference constructed", "chart_ref", chartRef)
	return chartRef, nil
}

// LoadKGSTChart validates the kgst chart reference for CLI usage
func (c *Client) LoadKGSTChart(ctx context.Context, version string) (string, error) {
	if version == "" {
		version = c.kgstVersion
	}
	
	c.logger.Debug("validating kgst chart", "version", version, "configured_version", c.kgstVersion)
	
	chartRef, err := c.LoadChart(ctx, KGSTChartURL, version)
	if err != nil {
		return "", fmt.Errorf("validate kgst chart: %w", err)
	}
	
	c.logger.Info("kgst chart validated", "version", version, "configured_version", c.kgstVersion)
	return chartRef, nil
}
