package helm

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
)

// Release represents information about a Helm release
type Release struct {
	Name      string    `json:"name"`
	Namespace string    `json:"namespace"`
	Version   int       `json:"version"`
	Status    string    `json:"status"`
	Chart     string    `json:"chart"`
	Manifest  string    `json:"manifest"`
	Info      ReleaseInfo `json:"info"`
}

// ReleaseInfo contains details about a Helm release
type ReleaseInfo struct {
	Description string    `json:"description"`
	Notes       string    `json:"notes"`
	Status      string    `json:"status"`
}

// InstallOrUpgrade installs or upgrades a Helm chart using the helm CLI
func (c *Client) InstallOrUpgrade(ctx context.Context, releaseName string, chartRef string, values map[string]interface{}) (*Release, error) {
	if releaseName == "" {
		return nil, fmt.Errorf("release name is required")
	}
	if chartRef == "" {
		return nil, fmt.Errorf("chart reference is required")
	}
	if values == nil {
		return nil, fmt.Errorf("values are required")
	}

	c.logger.Info("starting Helm install/upgrade",
		"release_name", releaseName,
		"chart_ref", chartRef,
		"namespace", c.namespace)

	// Validate values before proceeding
	if err := c.ValidateKGSTValues(values); err != nil {
		return nil, fmt.Errorf("invalid values: %w", err)
	}

	// Check if release already exists and handle any stuck operations
	existingRelease := c.checkReleaseExists(releaseName)
	if existingRelease {
		c.logger.Debug("release already exists, will upgrade", "release_name", releaseName)

		// Check if release is stuck in a pending operation
		if err := c.checkAndRecoverPendingRelease(releaseName); err != nil {
			c.logger.Warn("failed to recover pending release, continuing anyway",
				"release_name", releaseName,
				"error", err)
		}
	}

	// Build values file
	valuesData, err := c.buildValuesData(values)
	if err != nil {
		return nil, fmt.Errorf("build values data: %w", err)
	}

	// Execute helm upgrade --install
	// Note: --create-namespace is intentionally omitted to respect namespace filter validation
	// The namespace must already exist and be validated before this point
	args := []string{
		"upgrade",
		"--install",
		releaseName,
		chartRef,
		"--namespace", c.namespace,
		"--wait",
		"--timeout", "5m",
		"--atomic",
		"--values", "-",
	}

	cmd := exec.CommandContext(ctx, "helm", args...)
	cmd.Stdin = strings.NewReader(valuesData)
	
	output, err := cmd.CombinedOutput()
	if err != nil {
		c.logger.Error("Helm install/upgrade failed",
			"release_name", releaseName,
			"chart_ref", chartRef,
			"namespace", c.namespace,
			"existing_release", existingRelease,
			"error", err,
			"output", string(output))

		// Try to extract more detailed error information
		detailedErr := c.parseCLIError(string(output))
		return nil, fmt.Errorf("helm install/upgrade failed: %w", detailedErr)
	}

	// Get release information
	release, err := c.getRelease(releaseName)
	if err != nil {
		c.logger.Error("failed to get release info after install", 
			"release_name", releaseName, 
			"error", err)
		return nil, fmt.Errorf("get release %s: %w", releaseName, err)
	}

	// Determine operation status
	var status string
	if existingRelease {
		status = "updated"
		c.logger.Info("release updated successfully",
			"release_name", releaseName,
			"release_version", release.Version,
			"revision", strconv.Itoa(release.Version))
	} else {
		status = "created"
		c.logger.Info("release created successfully",
			"release_name", releaseName,
			"release_version", release.Version,
			"chart_name", release.Chart)
	}

	c.logger.Info("install/upgrade completed",
		"release_name", release.Name,
		"status", status,
		"namespace", release.Namespace,
		"chart_name", release.Chart,
		"description", release.Info.Description,
		"notes", release.Info.Notes)

	return release, nil
}

// buildValuesData converts values map to YAML string for helm CLI
func (c *Client) buildValuesData(values map[string]interface{}) (string, error) {
	valuesJSON, err := json.Marshal(values)
	if err != nil {
		return "", fmt.Errorf("marshal values: %w", err)
	}
	
	// Convert JSON to YAML for helm CLI (helm accepts JSON values)
	return string(valuesJSON), nil
}

// checkReleaseExists checks if a helm release already exists
func (c *Client) checkReleaseExists(releaseName string) bool {
	cmd := exec.Command("helm", "list", 
		"--namespace", c.namespace, 
		"--filter", "^"+releaseName+"$",
		"--output", "json")
	
	output, err := cmd.CombinedOutput()
	if err != nil {
		// Helm returns non-zero if no releases found, which is expected
		return false
	}
	
	// Parse JSON output
	var releases []map[string]interface{}
	if err := json.Unmarshal(output, &releases); err != nil {
		c.logger.Debug("failed to parse helm list output", "error", err)
		return false
	}
	
	return len(releases) > 0
}

// checkAndRecoverPendingRelease checks if a release is stuck in a pending operation
// and automatically recovers by rolling back to the last successful deployment
func (c *Client) checkAndRecoverPendingRelease(releaseName string) error {
	// Get release history
	cmd := exec.Command("helm", "history",
		releaseName,
		"--namespace", c.namespace,
		"--output", "json")

	output, err := cmd.CombinedOutput()
	if err != nil {
		// If we can't get history, the release might not exist yet
		return nil
	}

	// Parse history
	type historyEntry struct {
		Revision    int    `json:"revision"`
		Status      string `json:"status"`
		Description string `json:"description"`
	}

	var history []historyEntry
	if err := json.Unmarshal(output, &history); err != nil {
		return fmt.Errorf("parse history: %w", err)
	}

	if len(history) == 0 {
		return nil
	}

	// Check if the latest revision is in a pending state
	latest := history[len(history)-1]
	isPending := strings.HasPrefix(latest.Status, "pending-")

	if !isPending {
		c.logger.Debug("release is not in pending state",
			"release_name", releaseName,
			"status", latest.Status)
		return nil
	}

	c.logger.Warn("release is stuck in pending state, attempting recovery",
		"release_name", releaseName,
		"current_revision", latest.Revision,
		"status", latest.Status)

	// Find the last successful deployment
	var lastGoodRevision int
	for i := len(history) - 1; i >= 0; i-- {
		if history[i].Status == "deployed" || history[i].Status == "superseded" {
			lastGoodRevision = history[i].Revision
			break
		}
	}

	if lastGoodRevision == 0 {
		return fmt.Errorf("no successful deployment found to rollback to")
	}

	// Rollback to last good revision
	c.logger.Info("rolling back to last successful deployment",
		"release_name", releaseName,
		"target_revision", lastGoodRevision,
		"current_revision", latest.Revision)

	rollbackCmd := exec.Command("helm", "rollback",
		releaseName,
		fmt.Sprintf("%d", lastGoodRevision),
		"--namespace", c.namespace,
		"--wait")

	rollbackOutput, err := rollbackCmd.CombinedOutput()
	if err != nil {
		c.logger.Error("rollback failed",
			"release_name", releaseName,
			"target_revision", lastGoodRevision,
			"error", err,
			"output", string(rollbackOutput))
		return fmt.Errorf("rollback to revision %d: %w", lastGoodRevision, err)
	}

	c.logger.Info("successfully recovered pending release",
		"release_name", releaseName,
		"recovered_to_revision", lastGoodRevision)

	return nil
}

// getRelease gets helm release information
func (c *Client) getRelease(releaseName string) (*Release, error) {
	// Get status information with JSON output (this has all the metadata)
	statusCmd := exec.Command("helm", "status",
		releaseName,
		"--namespace", c.namespace,
		"--output", "json")

	statusOutput, err := statusCmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("helm status: %w, output: %s", err, string(statusOutput))
	}

	// Parse the JSON status output
	var statusData struct {
		Name      string `json:"name"`
		Namespace string `json:"namespace"`
		Version   int    `json:"version"`
		Info      struct {
			Status      string `json:"status"`
			Description string `json:"description"`
			Notes       string `json:"notes"`
		} `json:"info"`
		Chart struct {
			Metadata struct {
				Name    string `json:"name"`
				Version string `json:"version"`
			} `json:"metadata"`
		} `json:"chart"`
	}

	if err := json.Unmarshal(statusOutput, &statusData); err != nil {
		return nil, fmt.Errorf("parse status json: %w", err)
	}

	// Get the manifest (plain text, no JSON output option)
	manifestCmd := exec.Command("helm", "get",
		"manifest",
		releaseName,
		"--namespace", c.namespace)

	manifestOutput, err := manifestCmd.CombinedOutput()
	if err != nil {
		c.logger.Warn("failed to get manifest, continuing without it",
			"release_name", releaseName,
			"error", err)
		// Don't fail if we can't get the manifest, status is more important
	}

	// Build Release struct from the data
	release := &Release{
		Name:      statusData.Name,
		Namespace: statusData.Namespace,
		Version:   statusData.Version,
		Status:    statusData.Info.Status,
		Chart:     fmt.Sprintf("%s-%s", statusData.Chart.Metadata.Name, statusData.Chart.Metadata.Version),
		Manifest:  string(manifestOutput),
		Info: ReleaseInfo{
			Status:      statusData.Info.Status,
			Description: statusData.Info.Description,
			Notes:       statusData.Info.Notes,
		},
	}

	return release, nil
}

// parseCLIError parses helm CLI output to extract meaningful error information
func (c *Client) parseCLIError(output string) error {
	errStr := strings.TrimSpace(output)
	if errStr == "" {
		return fmt.Errorf("helm failed with no error output")
	}

	// Common error patterns
	switch {
	case strings.Contains(errStr, "another operation") && strings.Contains(errStr, "is in progress"):
		return fmt.Errorf("helm release is locked by another operation (this should be auto-recovered): %s", errStr)

	case strings.Contains(errStr, "chart not found"):
		return fmt.Errorf("chart not found in repository: %s", errStr)

	case strings.Contains(errStr, "authentication failed"):
		return fmt.Errorf("authentication failed when pulling chart: %s", errStr)

	case strings.Contains(errStr, "connection refused"):
		return fmt.Errorf("cannot connect to registry: %s", errStr)

	case strings.Contains(errStr, "Job failed") && strings.Contains(errStr, "verify-job"):
		// Extract verification job failure information
		return fmt.Errorf("chart verification failed: %s", errStr)

	case strings.Contains(errStr, "Validation webhook"):
		// Extract validation webhook rejection
		return fmt.Errorf("validation rejected: %s", errStr)

	case strings.Contains(errStr, "hook delete failed"):
		// Hook deletion failures are often transient
		return fmt.Errorf("hook cleanup failed: %s", errStr)

	case strings.Contains(errStr, "release already exists"):
		return fmt.Errorf("release conflict: %s", errStr)

	default:
		return fmt.Errorf("%s", errStr)
	}
}

// ExtractAppliedResources extracts the names of resources created or updated by a Helm release
func (c *Client) ExtractAppliedResources(release *Release) []string {
	var resources []string
	
	if release == nil {
		return resources
	}

	// Parse release manifest to extract resource information
	manifest := release.Manifest
	if manifest == "" {
		c.logger.Warn("release has no manifest", "release_name", release.Name)
		return resources
	}

	// Split manifest into individual YAML documents
	documents := strings.Split(manifest, "---")
	
	for _, doc := range documents {
		doc = strings.TrimSpace(doc)
		if doc == "" {
			continue
		}

		// Extract resource information from the YAML document
		// We'll parse using a simple approach since we don't have direct access
		// to the YAML unstructured parser from the Kubernetes client
		lines := strings.Split(doc, "\n")
		var kind, name, namespace string
		
		for _, line := range lines {
			line = strings.TrimSpace(line)
			if strings.HasPrefix(line, "kind:") {
				kind = strings.TrimPrefix(line, "kind:")
				kind = strings.TrimSpace(strings.Trim(kind, "\"'"))
			} else if strings.HasPrefix(line, "metadata:") {
				// Look for name and namespace in the next few lines
				continue
			} else if strings.HasPrefix(line, "name:") {
				name = strings.TrimPrefix(line, "name:")
				name = strings.TrimSpace(strings.Trim(name, "\"'"))
			} else if strings.HasPrefix(line, "namespace:") {
				namespace = strings.TrimPrefix(line, "namespace:")
				namespace = strings.TrimSpace(strings.Trim(namespace, "\"'"))
			}
		}

		// If we couldn't extract namespace from the document, use the release namespace
		if namespace == "" {
			namespace = release.Namespace
		}

		if kind != "" && name != "" && namespace != "" {
			resources = append(resources, fmt.Sprintf("%s/%s/%s", namespace, kind, name))
			c.logger.Debug("extracted resource from release",
				"resource", fmt.Sprintf("%s/%s/%s", namespace, kind, name),
				"release", release.Name)
		}
	}

	c.logger.Debug("extracted resources from release",
		"release_name", release.Name,
		"resource_count", len(resources))

	return resources
}

// ListReleases lists all Helm releases in the configured namespace
func (c *Client) ListReleases(ctx context.Context) ([]*Release, error) {
	c.logger.Debug("listing Helm releases", "namespace", c.namespace)

	cmd := exec.Command("helm", "list", 
		"--namespace", c.namespace, 
		"--output", "json")
	
	output, err := cmd.CombinedOutput()
	if err != nil {
		c.logger.Error("failed to list Helm releases", "namespace", c.namespace, "error", err)
		return nil, fmt.Errorf("list releases: %w, output: %s", err, string(output))
	}

	var releases []*Release
	if err := json.Unmarshal(output, &releases); err != nil {
		return nil, fmt.Errorf("parse releases json: %w", err)
	}

	c.logger.Debug("listed releases", "namespace", c.namespace, "count", len(releases))
	return releases, nil
}

// GetRelease gets information about a specific Helm release
func (c *Client) GetRelease(ctx context.Context, releaseName string) (*Release, error) {
	c.logger.Debug("getting Helm release", "release_name", releaseName, "namespace", c.namespace)

	release, err := c.getRelease(releaseName)
	if err != nil {
		c.logger.Error("failed to get Helm release", "release_name", releaseName, "namespace", c.namespace, "error", err)
		return nil, fmt.Errorf("get release %s: %w", releaseName, err)
	}

	c.logger.Debug("got Helm release", "release_name", releaseName, "status", release.Info.Status)
	return release, nil
}
