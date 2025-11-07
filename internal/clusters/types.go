package clusters

import (
	"time"
)

// CredentialSummary captures key metadata about a Credential resource.
type CredentialSummary struct {
	Name      string            `json:"name"`
	Namespace string            `json:"namespace"`
	Provider  string            `json:"provider,omitempty"`
	Labels    map[string]string `json:"labels,omitempty"`
	CreatedAt time.Time         `json:"created_at"`
	Ready     bool              `json:"ready"`
}

// ClusterTemplateSummary captures key metadata about a ClusterTemplate resource.
type ClusterTemplateSummary struct {
	Name        string            `json:"name"`
	Namespace   string            `json:"namespace"`
	Description string            `json:"description,omitempty"`
	Provider    string            `json:"provider,omitempty"`
	Cloud       string            `json:"cloud,omitempty"`
	Version     string            `json:"version,omitempty"`
	Labels      map[string]string `json:"labels,omitempty"`
	CreatedAt   time.Time         `json:"created_at"`
}

// DeployRequest specifies parameters for deploying a new ClusterDeployment.
type DeployRequest struct {
	// Name is the desired name of the ClusterDeployment
	Name string `json:"name"`

	// Template is the name (or namespace/name) of the ClusterTemplate to use
	Template string `json:"template"`

	// Credential is the name (or namespace/name) of the Credential to use
	Credential string `json:"credential"`

	// Namespace is the target namespace for the ClusterDeployment (optional, resolved per auth mode)
	Namespace string `json:"namespace,omitempty"`

	// Labels are additional labels to apply to the ClusterDeployment
	Labels map[string]string `json:"labels,omitempty"`

	// Config is the arbitrary configuration object passed to spec.config
	Config map[string]interface{} `json:"config,omitempty"`
}

// DeployResult reports the outcome of a cluster deployment operation.
type DeployResult struct {
	// Name of the ClusterDeployment
	Name string `json:"name"`

	// Namespace where the ClusterDeployment was created
	Namespace string `json:"namespace"`

	// UID of the created/updated resource
	UID string `json:"uid"`

	// Status indicates whether the resource was "created" or "updated"
	Status string `json:"status"`
}

// DeleteRequest specifies parameters for deleting a ClusterDeployment.
type DeleteRequest struct {
	// Name is the name of the ClusterDeployment to delete
	Name string `json:"name"`

	// Namespace is the namespace of the ClusterDeployment (optional, resolved per auth mode)
	Namespace string `json:"namespace,omitempty"`
}

// DeleteResult reports the outcome of a cluster deletion operation.
type DeleteResult struct {
	// Name of the ClusterDeployment
	Name string `json:"name"`

	// Namespace where the ClusterDeployment was deleted
	Namespace string `json:"namespace"`

	// Status indicates "deleted" or "not_found" (idempotent)
	Status string `json:"status"`
}

// ClusterDeploymentSummary captures key metadata about a ClusterDeployment resource.
type ClusterDeploymentSummary struct {
	Name      string            `json:"name"`
	Namespace string            `json:"namespace"`
	Template  string            `json:"template,omitempty"`
	Labels    map[string]string `json:"labels,omitempty"`
	Ready     bool              `json:"ready"`
	CreatedAt time.Time         `json:"createdAt"`
}
