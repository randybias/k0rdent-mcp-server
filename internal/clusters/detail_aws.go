package clusters

import (
	"context"
	"fmt"

	"github.com/k0rdent/mcp-k0rdent-server/internal/logging"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

var (
	// AWSClusterGVR is the GroupVersionResource for AWSCluster CRs from CAPI AWS provider
	AWSClusterGVR = schema.GroupVersionResource{
		Group:    "infrastructure.cluster.x-k8s.io",
		Version:  "v1beta2",
		Resource: "awsclusters",
	}
)

// GetAWSClusterDetail retrieves provider-specific infrastructure details for an AWS cluster.
// It fetches both the ClusterDeployment and the corresponding AWSCluster CR,
// returning deep inspection data including VPC, subnets, security groups, load balancers, etc.
func (m *Manager) GetAWSClusterDetail(ctx context.Context, namespace, name string) (AWSClusterDetail, error) {
	logger := logging.WithContext(ctx, m.logger)
	logger.Info("getting AWS cluster detail",
		"name", name,
		"namespace", namespace,
	)

	// Fetch the ClusterDeployment
	cdObj, err := m.dynamicClient.Resource(ClusterDeploymentsGVR).Namespace(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		if apierrors.IsNotFound(err) {
			logger.Warn("cluster deployment not found",
				"name", name,
				"namespace", namespace,
			)
			return AWSClusterDetail{}, fmt.Errorf("cluster deployment %s not found in namespace %s", name, namespace)
		}
		logger.Error("failed to fetch cluster deployment",
			"name", name,
			"namespace", namespace,
			"error", err,
		)
		return AWSClusterDetail{}, fmt.Errorf("fetch cluster deployment: %w", err)
	}

	// Extract basic metadata from ClusterDeployment
	detail := AWSClusterDetail{
		Name:         cdObj.GetName(),
		Namespace:    cdObj.GetNamespace(),
		TemplateRef:  buildTemplateReference(cdObj, namespace),
		CredentialRef: buildReferenceFromPath(cdObj, namespace, "spec", "credential"),
		Provider:     "aws",
	}

	// Infer region from ClusterDeployment config
	if region, found, err := unstructured.NestedString(cdObj.Object, "spec", "config", "region"); err == nil && found && region != "" {
		detail.Region = region
	}

	// Extract kubeconfig secret reference from status
	kubeconfigRef := buildReferenceFromPath(cdObj, namespace, "status", "kubeconfigSecret")
	if kubeconfigRef.Name != "" {
		detail.KubeconfigSecret = &kubeconfigRef
	}

	// Fetch the AWSCluster CR
	// The AWSCluster is typically named the same as the ClusterDeployment
	awsClusterObj, err := m.dynamicClient.Resource(AWSClusterGVR).Namespace(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		if apierrors.IsNotFound(err) {
			logger.Warn("AWSCluster CR not found",
				"name", name,
				"namespace", namespace,
			)
			return AWSClusterDetail{}, fmt.Errorf("AWSCluster CR %s not found in namespace %s (cluster may not be AWS or not yet provisioned)", name, namespace)
		}
		logger.Error("failed to fetch AWSCluster CR",
			"name", name,
			"namespace", namespace,
			"error", err,
		)
		return AWSClusterDetail{}, fmt.Errorf("fetch AWSCluster CR: %w", err)
	}

	// Extract AWS-specific infrastructure details
	detail.AWS = extractAWSInfrastructure(awsClusterObj)

	// Extract control plane endpoint from AWSCluster status
	if endpoint := extractAWSControlPlaneEndpoint(awsClusterObj); endpoint != nil {
		detail.ControlPlaneEndpoint = endpoint
	}

	// Extract provider-specific conditions from AWSCluster status
	detail.Conditions = extractConditions(awsClusterObj)

	logger.Info("AWS cluster detail retrieved",
		"name", name,
		"namespace", namespace,
		"vpc_id", detail.AWS.VPC.ID,
		"subnet_count", len(detail.AWS.Subnets),
	)

	return detail, nil
}

// extractAWSInfrastructure extracts AWS-specific infrastructure details from the AWSCluster CR.
func extractAWSInfrastructure(obj *unstructured.Unstructured) AWSInfrastructure {
	infra := AWSInfrastructure{}

	// Extract region from spec
	if region, found, err := unstructured.NestedString(obj.Object, "spec", "region"); err == nil && found {
		infra.Region = region
	}

	// Extract account ID from spec (optional)
	if accountID, found, err := unstructured.NestedString(obj.Object, "spec", "identityRef", "accountID"); err == nil && found {
		infra.AccountID = accountID
	}

	// Extract VPC information
	infra.VPC = extractAWSVPC(obj)

	// Extract subnets
	infra.Subnets = extractAWSSubnets(obj)

	// Extract internet gateway
	if igw := extractAWSInternetGateway(obj); igw != nil {
		infra.InternetGateway = igw
	}

	// Extract NAT gateways
	infra.NATGateways = extractAWSNATGateways(obj)

	// Extract load balancers
	infra.LoadBalancers = extractAWSLoadBalancers(obj)

	// Extract security groups
	infra.SecurityGroups = extractAWSSecurityGroups(obj)

	// Extract IAM roles
	infra.IAMRoles = extractAWSIAMRoles(obj)

	return infra
}

// extractAWSVPC extracts VPC information from the AWSCluster spec.
func extractAWSVPC(obj *unstructured.Unstructured) *AWSVPC {
	vpcID, found, err := unstructured.NestedString(obj.Object, "spec", "network", "vpc", "id")
	if err != nil || !found || vpcID == "" {
		return nil
	}

	vpc := &AWSVPC{
		ID: vpcID,
	}

	// Extract CIDR if available
	if cidr, found, err := unstructured.NestedString(obj.Object, "spec", "network", "vpc", "cidrBlock"); err == nil && found {
		vpc.CIDR = cidr
	}

	return vpc
}

// extractAWSSubnets extracts subnet information from the AWSCluster spec.
func extractAWSSubnets(obj *unstructured.Unstructured) []AWSSubnet {
	subnetList, found, err := unstructured.NestedSlice(obj.Object, "spec", "network", "subnets")
	if err != nil || !found || len(subnetList) == 0 {
		return nil
	}

	subnets := make([]AWSSubnet, 0, len(subnetList))
	for _, item := range subnetList {
		subnetMap, ok := item.(map[string]interface{})
		if !ok {
			continue
		}

		subnet := AWSSubnet{}

		if id, ok := subnetMap["resourceID"].(string); ok {
			subnet.ID = id
		}
		if id, ok := subnetMap["id"].(string); ok && subnet.ID == "" {
			subnet.ID = id
		}

		if cidr, ok := subnetMap["cidrBlock"].(string); ok {
			subnet.CIDR = cidr
		}

		if az, ok := subnetMap["availabilityZone"].(string); ok {
			subnet.AvailabilityZone = az
		}

		if isPublic, ok := subnetMap["isPublic"].(bool); ok {
			subnet.IsPublic = isPublic
		}

		// Infer role from tags or name
		if tags, ok := subnetMap["tags"].(map[string]interface{}); ok {
			if role, ok := tags["kubernetes.io/role"].(string); ok {
				subnet.Role = role
			}
		}

		// Only add subnets with valid IDs
		if subnet.ID != "" {
			subnets = append(subnets, subnet)
		}
	}

	return subnets
}

// extractAWSInternetGateway extracts internet gateway information from the AWSCluster spec.
func extractAWSInternetGateway(obj *unstructured.Unstructured) *AWSInternetGateway {
	igwID, found, err := unstructured.NestedString(obj.Object, "spec", "network", "internetGatewayId")
	if err != nil || !found || igwID == "" {
		return nil
	}

	return &AWSInternetGateway{
		ID: igwID,
	}
}

// extractAWSNATGateways extracts NAT gateway information from the AWSCluster spec.
func extractAWSNATGateways(obj *unstructured.Unstructured) []AWSNATGateway {
	natList, found, err := unstructured.NestedSlice(obj.Object, "spec", "network", "natGateways")
	if err != nil || !found || len(natList) == 0 {
		return nil
	}

	gateways := make([]AWSNATGateway, 0, len(natList))
	for _, item := range natList {
		natMap, ok := item.(map[string]interface{})
		if !ok {
			continue
		}

		gateway := AWSNATGateway{}

		if id, ok := natMap["id"].(string); ok {
			gateway.ID = id
		}
		if id, ok := natMap["natGatewayId"].(string); ok && gateway.ID == "" {
			gateway.ID = id
		}

		if subnetID, ok := natMap["subnetID"].(string); ok {
			gateway.SubnetID = subnetID
		}

		if az, ok := natMap["availabilityZone"].(string); ok {
			gateway.AvailabilityZone = az
		}

		if gateway.ID != "" {
			gateways = append(gateways, gateway)
		}
	}

	return gateways
}

// extractAWSLoadBalancers extracts load balancer information from the AWSCluster status.
func extractAWSLoadBalancers(obj *unstructured.Unstructured) []AWSLoadBalancer {
	// Control plane load balancer from status
	lbList := []AWSLoadBalancer{}

	// Check for control plane load balancer in status
	if lb := extractControlPlaneLoadBalancer(obj); lb != nil {
		lbList = append(lbList, *lb)
	}

	// Check for additional load balancers in spec
	additionalLBs, found, err := unstructured.NestedSlice(obj.Object, "spec", "controlPlaneLoadBalancer", "additionalListeners")
	if err == nil && found {
		for _, item := range additionalLBs {
			lbMap, ok := item.(map[string]interface{})
			if !ok {
				continue
			}

			lb := AWSLoadBalancer{}
			if arn, ok := lbMap["arn"].(string); ok {
				lb.ARN = arn
			}
			if name, ok := lbMap["name"].(string); ok {
				lb.Name = name
			}
			if lb.ARN != "" || lb.Name != "" {
				lbList = append(lbList, lb)
			}
		}
	}

	return lbList
}

// extractControlPlaneLoadBalancer extracts the control plane load balancer from status.
func extractControlPlaneLoadBalancer(obj *unstructured.Unstructured) *AWSLoadBalancer {
	// Try to get from spec first
	scheme, _, _ := unstructured.NestedString(obj.Object, "spec", "controlPlaneLoadBalancer", "scheme")
	lbName, _, _ := unstructured.NestedString(obj.Object, "spec", "controlPlaneLoadBalancer", "name")

	// Try to get from status
	dnsName, _, _ := unstructured.NestedString(obj.Object, "status", "network", "apiServerElb", "dnsName")
	if dnsName == "" {
		return nil
	}

	lb := &AWSLoadBalancer{
		DNSName: dnsName,
		Scheme:  scheme,
		Type:    "classic", // CAPI AWS typically uses classic ELB for control plane
	}

	if lbName != "" {
		lb.Name = lbName
	}

	return lb
}

// extractAWSSecurityGroups extracts security group information from the AWSCluster spec.
func extractAWSSecurityGroups(obj *unstructured.Unstructured) []AWSSecurityGroup {
	sgList, found, err := unstructured.NestedSlice(obj.Object, "spec", "network", "securityGroupOverrides")
	if err != nil || !found || len(sgList) == 0 {
		// Try alternative path
		sgMap, found, err := unstructured.NestedMap(obj.Object, "status", "network", "securityGroups")
		if err != nil || !found {
			return nil
		}

		groups := []AWSSecurityGroup{}
		for name, value := range sgMap {
			if idMap, ok := value.(map[string]interface{}); ok {
				if id, ok := idMap["id"].(string); ok {
					groups = append(groups, AWSSecurityGroup{
						ID:   id,
						Name: name,
					})
				}
			}
		}
		return groups
	}

	groups := make([]AWSSecurityGroup, 0, len(sgList))
	for _, item := range sgList {
		sgMap, ok := item.(map[string]interface{})
		if !ok {
			continue
		}

		sg := AWSSecurityGroup{}
		if id, ok := sgMap["id"].(string); ok {
			sg.ID = id
		}
		if name, ok := sgMap["name"].(string); ok {
			sg.Name = name
		}

		if sg.ID != "" {
			groups = append(groups, sg)
		}
	}

	return groups
}

// extractAWSIAMRoles extracts IAM role information from the AWSCluster spec.
func extractAWSIAMRoles(obj *unstructured.Unstructured) []AWSIAMRole {
	roles := []AWSIAMRole{}

	// Control plane IAM role
	if arn, found, err := unstructured.NestedString(obj.Object, "spec", "controlPlaneIAMInstanceProfile"); err == nil && found && arn != "" {
		roles = append(roles, AWSIAMRole{
			ARN:  arn,
			Role: "control-plane",
		})
	}

	// Node IAM role (if specified)
	if arn, found, err := unstructured.NestedString(obj.Object, "spec", "nodeIAMInstanceProfile"); err == nil && found && arn != "" {
		roles = append(roles, AWSIAMRole{
			ARN:  arn,
			Role: "worker",
		})
	}

	return roles
}

// extractAWSControlPlaneEndpoint extracts the control plane endpoint from the AWSCluster status.
func extractAWSControlPlaneEndpoint(obj *unstructured.Unstructured) *EndpointInfo {
	host, found, err := unstructured.NestedString(obj.Object, "spec", "controlPlaneEndpoint", "host")
	if err != nil || !found || host == "" {
		// Try status path
		host, found, err = unstructured.NestedString(obj.Object, "status", "ready")
		if err != nil || !found {
			return nil
		}
	}

	endpoint := &EndpointInfo{
		Host: host,
	}

	// Extract port if available
	if portFloat, found, err := unstructured.NestedFloat64(obj.Object, "spec", "controlPlaneEndpoint", "port"); err == nil && found {
		endpoint.Port = int32(portFloat)
	}

	return endpoint
}
