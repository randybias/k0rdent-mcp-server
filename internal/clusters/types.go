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

// ProviderSummary lists supported infrastructure providers.
type ProviderSummary struct {
	Name        string `json:"name"`
	Title       string `json:"title"`
	Description string `json:"description,omitempty"`
}

// IdentitySummary links ClusterIdentity resources to the Credentials that reference them.
type IdentitySummary struct {
	Name        string   `json:"name"`
	Namespace   string   `json:"namespace"`
	Kind        string   `json:"kind,omitempty"`
	Provider    string   `json:"provider,omitempty"`
	Credentials []string `json:"credentials,omitempty"`
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
	Name               string             `json:"name"`
	Namespace          string             `json:"namespace"`
	Labels             map[string]string  `json:"labels,omitempty"`
	Owner              string             `json:"owner,omitempty"`
	CreatedAt          time.Time          `json:"createdAt"`
	AgeSeconds         int64              `json:"ageSeconds,omitempty"`
	TemplateRef        ResourceReference  `json:"templateRef"`
	CredentialRef      ResourceReference  `json:"credentialRef"`
	ClusterIdentityRef ResourceReference  `json:"clusterIdentityRef,omitempty"`
	ServiceTemplates   []string           `json:"serviceTemplates,omitempty"`
	CloudProvider      string             `json:"cloudProvider,omitempty"`
	Region             string             `json:"region,omitempty"`
	Ready              bool               `json:"ready"`
	Phase              string             `json:"phase,omitempty"`
	Message            string             `json:"message,omitempty"`
	Conditions         []ConditionSummary `json:"conditions,omitempty"`
	KubeconfigSecret   ResourceReference  `json:"kubeconfigSecret,omitempty"`
	ManagementURL      string             `json:"managementURL,omitempty"`
}

// ResourceReference describes a related Kubernetes resource.
type ResourceReference struct {
	Name      string `json:"name"`
	Namespace string `json:"namespace,omitempty"`
	Version   string `json:"version,omitempty"`
}

// ConditionSummary captures a simplified view of a Kubernetes condition.
type ConditionSummary struct {
	Type               string     `json:"type"`
	Status             string     `json:"status"`
	Reason             string     `json:"reason,omitempty"`
	Message            string     `json:"message,omitempty"`
	LastTransitionTime *time.Time `json:"lastTransitionTime,omitempty"`
}

// AzureClusterDetail provides deep Azure infrastructure inspection for a ClusterDeployment.
type AzureClusterDetail struct {
	// Basic cluster metadata (consistent with getState)
	Name         string            `json:"name"`
	Namespace    string            `json:"namespace"`
	TemplateRef  ResourceReference `json:"templateRef"`
	CredentialRef ResourceReference `json:"credentialRef"`
	Provider     string            `json:"provider"`
	Region       string            `json:"region"`

	// Azure-specific infrastructure details
	Azure AzureInfrastructure `json:"azure"`

	// Control plane endpoint
	ControlPlaneEndpoint *EndpointInfo `json:"controlPlaneEndpoint,omitempty"`

	// Kubeconfig secret reference
	KubeconfigSecret *ResourceReference `json:"kubeconfigSecret,omitempty"`

	// Provider-specific conditions
	Conditions []ConditionSummary `json:"conditions,omitempty"`
}

// AzureInfrastructure contains Azure-specific resource IDs and topology.
type AzureInfrastructure struct {
	ResourceGroup  string            `json:"resourceGroup"`
	SubscriptionID string            `json:"subscriptionID"`
	Location       string            `json:"location"`
	IdentityRef    *ResourceReference `json:"identityRef,omitempty"`

	// Network infrastructure
	VNet    *AzureVNet    `json:"vnet,omitempty"`
	Subnets []AzureSubnet `json:"subnets,omitempty"`

	// Optional network components
	NATGateway    *AzureNATGateway    `json:"natGateway,omitempty"`
	LoadBalancers []AzureLoadBalancer `json:"loadBalancers,omitempty"`
	SecurityGroups []AzureSecurityGroup `json:"securityGroups,omitempty"`
}

// AzureVNet represents an Azure Virtual Network.
type AzureVNet struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	CIDR string `json:"cidr,omitempty"`
}

// AzureSubnet represents an Azure subnet.
type AzureSubnet struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	CIDR string `json:"cidr,omitempty"`
	Role string `json:"role,omitempty"` // e.g., "control-plane", "worker"
}

// AzureNATGateway represents an Azure NAT Gateway.
type AzureNATGateway struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// AzureLoadBalancer represents an Azure Load Balancer.
type AzureLoadBalancer struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Type     string `json:"type,omitempty"` // "public", "internal"
	FrontendIP string `json:"frontendIP,omitempty"`
}

// AzureSecurityGroup represents an Azure Network Security Group.
type AzureSecurityGroup struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// AWSClusterDetail provides deep AWS infrastructure inspection for a ClusterDeployment.
type AWSClusterDetail struct {
	// Basic cluster metadata (consistent with getState)
	Name         string            `json:"name"`
	Namespace    string            `json:"namespace"`
	TemplateRef  ResourceReference `json:"templateRef"`
	CredentialRef ResourceReference `json:"credentialRef"`
	Provider     string            `json:"provider"`
	Region       string            `json:"region"`

	// AWS-specific infrastructure details
	AWS AWSInfrastructure `json:"aws"`

	// Control plane endpoint
	ControlPlaneEndpoint *EndpointInfo `json:"controlPlaneEndpoint,omitempty"`

	// Kubeconfig secret reference
	KubeconfigSecret *ResourceReference `json:"kubeconfigSecret,omitempty"`

	// Provider-specific conditions
	Conditions []ConditionSummary `json:"conditions,omitempty"`
}

// AWSInfrastructure contains AWS-specific resource IDs and topology.
type AWSInfrastructure struct {
	AccountID string `json:"accountID,omitempty"`
	Region    string `json:"region"`

	// Network infrastructure
	VPC     *AWSVPC     `json:"vpc,omitempty"`
	Subnets []AWSSubnet `json:"subnets,omitempty"`

	// Optional network components
	InternetGateway *AWSInternetGateway `json:"internetGateway,omitempty"`
	NATGateways     []AWSNATGateway     `json:"natGateways,omitempty"`
	LoadBalancers   []AWSLoadBalancer   `json:"loadBalancers,omitempty"`
	SecurityGroups  []AWSSecurityGroup  `json:"securityGroups,omitempty"`

	// IAM
	IAMRoles []AWSIAMRole `json:"iamRoles,omitempty"`
}

// AWSVPC represents an AWS VPC.
type AWSVPC struct {
	ID   string `json:"id"`
	CIDR string `json:"cidr,omitempty"`
}

// AWSSubnet represents an AWS subnet.
type AWSSubnet struct {
	ID               string `json:"id"`
	CIDR             string `json:"cidr,omitempty"`
	AvailabilityZone string `json:"availabilityZone,omitempty"`
	IsPublic         bool   `json:"isPublic,omitempty"`
	Role             string `json:"role,omitempty"` // e.g., "control-plane", "worker"
}

// AWSInternetGateway represents an AWS Internet Gateway.
type AWSInternetGateway struct {
	ID string `json:"id"`
}

// AWSNATGateway represents an AWS NAT Gateway.
type AWSNATGateway struct {
	ID               string `json:"id"`
	SubnetID         string `json:"subnetID,omitempty"`
	AvailabilityZone string `json:"availabilityZone,omitempty"`
}

// AWSLoadBalancer represents an AWS ELB/ALB/NLB.
type AWSLoadBalancer struct {
	ARN      string `json:"arn"`
	Name     string `json:"name,omitempty"`
	Type     string `json:"type,omitempty"` // "classic", "application", "network"
	Scheme   string `json:"scheme,omitempty"` // "internet-facing", "internal"
	DNSName  string `json:"dnsName,omitempty"`
}

// AWSSecurityGroup represents an AWS Security Group.
type AWSSecurityGroup struct {
	ID   string `json:"id"`
	Name string `json:"name,omitempty"`
}

// AWSIAMRole represents an AWS IAM Role.
type AWSIAMRole struct {
	ARN  string `json:"arn"`
	Name string `json:"name,omitempty"`
	Role string `json:"role,omitempty"` // e.g., "control-plane", "worker"
}

// GCPClusterDetail provides deep GCP infrastructure inspection for a ClusterDeployment.
type GCPClusterDetail struct {
	// Basic cluster metadata (consistent with getState)
	Name         string            `json:"name"`
	Namespace    string            `json:"namespace"`
	TemplateRef  ResourceReference `json:"templateRef"`
	CredentialRef ResourceReference `json:"credentialRef"`
	Provider     string            `json:"provider"`
	Region       string            `json:"region"`

	// GCP-specific infrastructure details
	GCP GCPInfrastructure `json:"gcp"`

	// Control plane endpoint
	ControlPlaneEndpoint *EndpointInfo `json:"controlPlaneEndpoint,omitempty"`

	// Kubeconfig secret reference
	KubeconfigSecret *ResourceReference `json:"kubeconfigSecret,omitempty"`

	// Provider-specific conditions
	Conditions []ConditionSummary `json:"conditions,omitempty"`
}

// GCPInfrastructure contains GCP-specific resource IDs and topology.
type GCPInfrastructure struct {
	Project string `json:"project"`
	Region  string `json:"region"`

	// Network infrastructure
	Network *GCPNetwork `json:"network,omitempty"`
	Subnets []GCPSubnet `json:"subnets,omitempty"`

	// Optional network components
	FirewallRules []GCPFirewallRule `json:"firewallRules,omitempty"`
	Routers       []GCPRouter       `json:"routers,omitempty"`

	// Service accounts
	ServiceAccounts []GCPServiceAccount `json:"serviceAccounts,omitempty"`
}

// GCPNetwork represents a GCP VPC network.
type GCPNetwork struct {
	Name     string `json:"name"`
	SelfLink string `json:"selfLink,omitempty"`
}

// GCPSubnet represents a GCP subnet.
type GCPSubnet struct {
	Name         string `json:"name"`
	CIDR         string `json:"cidr,omitempty"`
	Region       string `json:"region,omitempty"`
	SelfLink     string `json:"selfLink,omitempty"`
	Role         string `json:"role,omitempty"` // e.g., "control-plane", "worker"
}

// GCPFirewallRule represents a GCP firewall rule.
type GCPFirewallRule struct {
	Name     string `json:"name"`
	SelfLink string `json:"selfLink,omitempty"`
}

// GCPRouter represents a GCP Cloud Router.
type GCPRouter struct {
	Name     string `json:"name"`
	SelfLink string `json:"selfLink,omitempty"`
}

// GCPServiceAccount represents a GCP service account.
type GCPServiceAccount struct {
	Email string `json:"email"`
	Name  string `json:"name,omitempty"`
	Role  string `json:"role,omitempty"` // e.g., "control-plane", "worker"
}

// EndpointInfo represents control plane endpoint details.
type EndpointInfo struct {
	Host string `json:"host"`
	Port int32  `json:"port,omitempty"`
}
