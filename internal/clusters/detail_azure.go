package clusters

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/k0rdent/mcp-k0rdent-server/internal/logging"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
)

var (
	// AzureClusterGVR is the GroupVersionResource for AzureCluster provider CRs
	AzureClusterGVR = schema.GroupVersionResource{
		Group:    "infrastructure.cluster.x-k8s.io",
		Version:  "v1beta1",
		Resource: "azureclusters",
	}
)

// GetAzureClusterDetail retrieves detailed Azure infrastructure information for a ClusterDeployment.
// It fetches both the ClusterDeployment CR and the corresponding AzureCluster provider CR.
func (m *Manager) GetAzureClusterDetail(ctx context.Context, namespace, name string) (*AzureClusterDetail, error) {
	logger := logging.WithContext(ctx, m.logger)
	logger.Debug("getting Azure cluster detail",
		"name", name,
		"namespace", namespace,
	)

	// Fetch ClusterDeployment
	clusterDeployment, err := m.dynamicClient.Resource(ClusterDeploymentsGVR).Namespace(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		logger.Error("failed to get ClusterDeployment",
			"name", name,
			"namespace", namespace,
			"error", err,
		)
		return nil, fmt.Errorf("get ClusterDeployment: %w", err)
	}

	logger.Debug("fetched ClusterDeployment", "name", name)

	// Extract basic metadata from ClusterDeployment
	summary := SummarizeClusterDeployment(clusterDeployment)

	// Fetch AzureCluster provider CR
	azureCluster, err := m.getAzureCluster(ctx, clusterDeployment, logger)
	if err != nil {
		logger.Error("failed to get AzureCluster",
			"name", name,
			"namespace", namespace,
			"error", err,
		)
		return nil, err
	}

	logger.Debug("fetched AzureCluster", "name", azureCluster.GetName())

	// Build Azure cluster detail
	detail := &AzureClusterDetail{
		Name:          summary.Name,
		Namespace:     summary.Namespace,
		TemplateRef:   summary.TemplateRef,
		CredentialRef: summary.CredentialRef,
		Provider:      "azure",
		Region:        summary.Region,
	}

	// Extract Azure-specific infrastructure
	detail.Azure = m.extractAzureInfrastructure(azureCluster)

	// Extract control plane endpoint
	detail.ControlPlaneEndpoint = extractControlPlaneEndpoint(azureCluster)

	// Extract kubeconfig secret reference
	if summary.KubeconfigSecret.Name != "" {
		detail.KubeconfigSecret = &summary.KubeconfigSecret
	}

	// Extract provider-specific conditions from AzureCluster
	detail.Conditions = extractConditions(azureCluster)

	logger.Info("Azure cluster detail retrieved",
		"name", name,
		"namespace", namespace,
		"resource_group", detail.Azure.ResourceGroup,
		"location", detail.Azure.Location,
	)

	return detail, nil
}

// getAzureCluster fetches the AzureCluster provider CR referenced by the ClusterDeployment
func (m *Manager) getAzureCluster(ctx context.Context, clusterDeployment *unstructured.Unstructured, logger *slog.Logger) (*unstructured.Unstructured, error) {
	// Try to find the AzureCluster name from spec.config.clusterNetwork.cluster or use the ClusterDeployment name
	azureClusterName := clusterDeployment.GetName()

	// Try alternative naming patterns (some CAPI setups use different naming)
	namespace := clusterDeployment.GetNamespace()

	// Attempt to fetch the AzureCluster
	azureCluster, err := m.dynamicClient.Resource(AzureClusterGVR).Namespace(namespace).Get(ctx, azureClusterName, metav1.GetOptions{})
	if err != nil {
		logger.Debug("AzureCluster not found with default name, trying to discover",
			"attempted_name", azureClusterName,
			"error", err,
		)

		// Try to discover the AzureCluster by listing and matching labels
		azureCluster, err = m.discoverAzureCluster(ctx, namespace, clusterDeployment)
		if err != nil {
			return nil, fmt.Errorf("AzureCluster not found for ClusterDeployment %s: %w", clusterDeployment.GetName(), err)
		}
	}

	return azureCluster, nil
}

// discoverAzureCluster attempts to find the AzureCluster by listing and matching labels
func (m *Manager) discoverAzureCluster(ctx context.Context, namespace string, clusterDeployment *unstructured.Unstructured) (*unstructured.Unstructured, error) {
	// List all AzureClusters in the namespace
	list, err := m.dynamicClient.Resource(AzureClusterGVR).Namespace(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("list AzureClusters: %w", err)
	}

	// Try to match by cluster.x-k8s.io/cluster-name label
	clusterName := clusterDeployment.GetName()
	for i := range list.Items {
		labels := list.Items[i].GetLabels()
		if labels != nil && labels["cluster.x-k8s.io/cluster-name"] == clusterName {
			return &list.Items[i], nil
		}
	}

	// If no match found, return error with descriptive message
	return nil, fmt.Errorf("no AzureCluster found matching ClusterDeployment %s in namespace %s", clusterName, namespace)
}

// extractAzureInfrastructure extracts Azure-specific infrastructure details from AzureCluster
func (m *Manager) extractAzureInfrastructure(azureCluster *unstructured.Unstructured) AzureInfrastructure {
	infra := AzureInfrastructure{}

	// Extract basic Azure configuration
	if resourceGroup, found, err := unstructured.NestedString(azureCluster.Object, "spec", "resourceGroup"); err == nil && found {
		infra.ResourceGroup = resourceGroup
	}

	if subscriptionID, found, err := unstructured.NestedString(azureCluster.Object, "spec", "subscriptionID"); err == nil && found {
		infra.SubscriptionID = subscriptionID
	}

	if location, found, err := unstructured.NestedString(azureCluster.Object, "spec", "location"); err == nil && found {
		infra.Location = location
	}

	// Extract identity reference
	if identityRef, found, err := unstructured.NestedMap(azureCluster.Object, "spec", "identityRef"); err == nil && found {
		infra.IdentityRef = &ResourceReference{
			Name:      asString(identityRef["name"]),
			Namespace: asString(identityRef["namespace"]),
		}
		if infra.IdentityRef.Name == "" {
			infra.IdentityRef = nil
		}
	}

	// Extract network infrastructure
	infra.VNet = extractAzureVNet(azureCluster)
	infra.Subnets = extractAzureSubnets(azureCluster)

	// Extract optional network components
	infra.NATGateway = extractAzureNATGateway(azureCluster)
	infra.LoadBalancers = extractAzureLoadBalancers(azureCluster)
	infra.SecurityGroups = extractAzureSecurityGroups(azureCluster)

	return infra
}

// extractAzureVNet extracts VNet information from AzureCluster
func extractAzureVNet(azureCluster *unstructured.Unstructured) *AzureVNet {
	vnetMap, found, err := unstructured.NestedMap(azureCluster.Object, "spec", "networkSpec", "vnet")
	if err != nil || !found {
		return nil
	}

	vnet := &AzureVNet{
		Name: asString(vnetMap["name"]),
		ID:   asString(vnetMap["id"]),
	}

	// Extract CIDR blocks
	if cidrBlocks, found, err := unstructured.NestedStringSlice(azureCluster.Object, "spec", "networkSpec", "vnet", "cidrBlocks"); err == nil && found && len(cidrBlocks) > 0 {
		vnet.CIDR = cidrBlocks[0]
	}

	if vnet.Name == "" && vnet.ID == "" {
		return nil
	}

	return vnet
}

// extractAzureSubnets extracts subnet information from AzureCluster
func extractAzureSubnets(azureCluster *unstructured.Unstructured) []AzureSubnet {
	subnetsList, found, err := unstructured.NestedSlice(azureCluster.Object, "spec", "networkSpec", "subnets")
	if err != nil || !found || len(subnetsList) == 0 {
		return nil
	}

	subnets := make([]AzureSubnet, 0, len(subnetsList))
	for _, item := range subnetsList {
		subnetMap, ok := item.(map[string]interface{})
		if !ok {
			continue
		}

		subnet := AzureSubnet{
			Name: asString(subnetMap["name"]),
			ID:   asString(subnetMap["id"]),
		}

		// Extract CIDR blocks
		if cidrBlocks, ok := subnetMap["cidrBlocks"].([]interface{}); ok && len(cidrBlocks) > 0 {
			if cidr, ok := cidrBlocks[0].(string); ok {
				subnet.CIDR = cidr
			}
		}

		// Infer role from subnet name or role field
		if role, ok := subnetMap["role"].(string); ok {
			subnet.Role = role
		} else {
			// Try to infer from name
			name := subnet.Name
			if name != "" {
				switch {
				case contains(name, "control"):
					subnet.Role = "control-plane"
				case contains(name, "worker"):
					subnet.Role = "worker"
				}
			}
		}

		if subnet.Name != "" || subnet.ID != "" {
			subnets = append(subnets, subnet)
		}
	}

	return subnets
}

// extractAzureNATGateway extracts NAT Gateway information from AzureCluster
func extractAzureNATGateway(azureCluster *unstructured.Unstructured) *AzureNATGateway {
	natMap, found, err := unstructured.NestedMap(azureCluster.Object, "spec", "networkSpec", "natGateway")
	if err != nil || !found {
		return nil
	}

	nat := &AzureNATGateway{
		Name: asString(natMap["name"]),
		ID:   asString(natMap["id"]),
	}

	if nat.Name == "" && nat.ID == "" {
		return nil
	}

	return nat
}

// extractAzureLoadBalancers extracts load balancer information from AzureCluster
func extractAzureLoadBalancers(azureCluster *unstructured.Unstructured) []AzureLoadBalancer {
	// Try to extract API server load balancer from status
	apiLBMap, found, err := unstructured.NestedMap(azureCluster.Object, "spec", "networkSpec", "apiServerLB")
	if err == nil && found {
		lb := AzureLoadBalancer{
			Name: asString(apiLBMap["name"]),
			ID:   asString(apiLBMap["id"]),
			Type: asString(apiLBMap["type"]),
		}

		// Try to get frontend IP from status
		if frontendIPs, found, err := unstructured.NestedSlice(azureCluster.Object, "spec", "networkSpec", "apiServerLB", "frontendIPs"); err == nil && found && len(frontendIPs) > 0 {
			if ipMap, ok := frontendIPs[0].(map[string]interface{}); ok {
				if publicIP, found, err := unstructured.NestedString(ipMap, "publicIP", "ipAddress"); err == nil && found {
					lb.FrontendIP = publicIP
				}
			}
		}

		if lb.Name != "" || lb.ID != "" {
			return []AzureLoadBalancer{lb}
		}
	}

	return nil
}

// extractAzureSecurityGroups extracts security group information from AzureCluster
func extractAzureSecurityGroups(azureCluster *unstructured.Unstructured) []AzureSecurityGroup {
	// Try to extract from subnets
	subnetsList, found, err := unstructured.NestedSlice(azureCluster.Object, "spec", "networkSpec", "subnets")
	if err != nil || !found || len(subnetsList) == 0 {
		return nil
	}

	sgMap := make(map[string]AzureSecurityGroup)
	for _, item := range subnetsList {
		subnetMap, ok := item.(map[string]interface{})
		if !ok {
			continue
		}

		// Extract security group from subnet
		if nsgMap, ok := subnetMap["securityGroup"].(map[string]interface{}); ok {
			id := asString(nsgMap["id"])
			name := asString(nsgMap["name"])
			if id != "" || name != "" {
				key := id
				if key == "" {
					key = name
				}
				sgMap[key] = AzureSecurityGroup{
					ID:   id,
					Name: name,
				}
			}
		}
	}

	if len(sgMap) == 0 {
		return nil
	}

	sgs := make([]AzureSecurityGroup, 0, len(sgMap))
	for _, sg := range sgMap {
		sgs = append(sgs, sg)
	}

	return sgs
}

// extractControlPlaneEndpoint extracts control plane endpoint from AzureCluster status
func extractControlPlaneEndpoint(azureCluster *unstructured.Unstructured) *EndpointInfo {
	host, found, err := unstructured.NestedString(azureCluster.Object, "spec", "controlPlaneEndpoint", "host")
	if err != nil || !found || host == "" {
		return nil
	}

	endpoint := &EndpointInfo{
		Host: host,
	}

	if port, found, err := unstructured.NestedInt64(azureCluster.Object, "spec", "controlPlaneEndpoint", "port"); err == nil && found {
		endpoint.Port = int32(port)
	}

	return endpoint
}

// contains checks if a string contains a substring (case-insensitive)
func contains(s, substr string) bool {
	return containsSubstr(toLower(s), toLower(substr))
}

// containsSubstr checks if s contains substr
func containsSubstr(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// ensureAzureClusterGVRAvailable checks if the dynamic client can access AzureClusters
func (m *Manager) ensureAzureClusterGVRAvailable(ctx context.Context) error {
	// Try to list AzureClusters to verify access
	_, err := m.dynamicClient.Resource(AzureClusterGVR).List(ctx, metav1.ListOptions{Limit: 1})
	return err
}

// GetAzureClusterDetailWithValidation retrieves Azure cluster detail with upfront validation
func (m *Manager) GetAzureClusterDetailWithValidation(ctx context.Context, namespace, name string) (*AzureClusterDetail, error) {
	// Validate AzureCluster GVR availability
	if err := m.ensureAzureClusterGVRAvailable(ctx); err != nil {
		return nil, fmt.Errorf("AzureCluster resource not available: %w", err)
	}

	return m.GetAzureClusterDetail(ctx, namespace, name)
}

// getResourceWithFallback attempts to get a resource from dynamic client with fallback strategies
func getResourceWithFallback(ctx context.Context, client dynamic.Interface, gvr schema.GroupVersionResource, namespace, name string) (*unstructured.Unstructured, error) {
	obj, err := client.Resource(gvr).Namespace(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}
	return obj, nil
}
