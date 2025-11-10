package clusters

import (
	"context"
	"fmt"

	"github.com/k0rdent/mcp-k0rdent-server/internal/logging"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

var (
	// GCPClusterGVR is the GroupVersionResource for GCPCluster CRs from CAPI GCP provider
	GCPClusterGVR = schema.GroupVersionResource{
		Group:    "infrastructure.cluster.x-k8s.io",
		Version:  "v1beta1",
		Resource: "gcpclusters",
	}
)

// GetGCPClusterDetail fetches detailed GCP infrastructure information for a ClusterDeployment.
// Returns GCPClusterDetail with provider-specific infrastructure details extracted from GCPCluster CR.
func (m *Manager) GetGCPClusterDetail(ctx context.Context, namespace, name string) (*GCPClusterDetail, error) {
	logger := logging.WithContext(ctx, m.logger)
	logger.Info("fetching GCP cluster detail",
		"name", name,
		"namespace", namespace,
	)

	// Fetch ClusterDeployment
	deployment, err := m.dynamicClient.Resource(ClusterDeploymentsGVR).Namespace(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		logger.Error("failed to fetch cluster deployment",
			"name", name,
			"namespace", namespace,
			"error", err,
		)
		return nil, fmt.Errorf("get cluster deployment %s/%s: %w", namespace, name, err)
	}

	// Extract basic metadata
	detail := &GCPClusterDetail{
		Name:      deployment.GetName(),
		Namespace: deployment.GetNamespace(),
	}

	// Extract template reference
	if templateName, found, _ := unstructured.NestedString(deployment.Object, "spec", "template"); found {
		detail.TemplateRef = ResourceReference{
			Name: templateName,
		}
	}

	// Extract credential reference
	if credentialName, found, _ := unstructured.NestedString(deployment.Object, "spec", "credential"); found {
		detail.CredentialRef = ResourceReference{
			Name: credentialName,
		}
	}

	// Extract provider and region from config
	if config, found, _ := unstructured.NestedMap(deployment.Object, "spec", "config"); found {
		if project, ok := config["project"].(string); ok {
			detail.GCP.Project = project
		}
		if region, ok := config["region"].(string); ok {
			detail.GCP.Region = region
			detail.Region = region
		}
	}

	detail.Provider = "gcp"

	// Get the GCPCluster name from status.clusterRef
	var gcpClusterName string
	if clusterRef, found, _ := unstructured.NestedMap(deployment.Object, "status", "clusterRef"); found {
		if kind, ok := clusterRef["kind"].(string); ok && kind == "GCPCluster" {
			if refName, ok := clusterRef["name"].(string); ok {
				gcpClusterName = refName
			}
		}
	}

	// If clusterRef not found, fall back to deployment name
	if gcpClusterName == "" {
		gcpClusterName = name
	}

	logger.Debug("resolved GCPCluster name", "gcpClusterName", gcpClusterName)

	// Fetch GCPCluster CR
	gcpCluster, err := m.dynamicClient.Resource(GCPClusterGVR).Namespace(namespace).Get(ctx, gcpClusterName, metav1.GetOptions{})
	if err != nil {
		logger.Error("failed to fetch GCPCluster",
			"name", gcpClusterName,
			"namespace", namespace,
			"error", err,
		)
		return nil, fmt.Errorf("GCPCluster %s/%s not found: %w", namespace, gcpClusterName, err)
	}

	logger.Debug("fetched GCPCluster", "name", gcpClusterName)

	// Extract GCP-specific infrastructure details from spec
	if spec, found, _ := unstructured.NestedMap(gcpCluster.Object, "spec"); found {
		// Extract project
		if project, ok := spec["project"].(string); ok {
			detail.GCP.Project = project
		}

		// Extract region
		if region, ok := spec["region"].(string); ok {
			detail.GCP.Region = region
			detail.Region = region
		}

		// Extract network infrastructure
		if network, ok := spec["network"].(map[string]interface{}); ok {
			if name, ok := network["name"].(string); ok {
				detail.GCP.Network = &GCPNetwork{
					Name: name,
				}
				if selfLink, ok := network["selfLink"].(string); ok {
					detail.GCP.Network.SelfLink = selfLink
				}
			}
		}

		// Extract subnets
		if subnets, ok := spec["subnets"].([]interface{}); ok {
			detail.GCP.Subnets = make([]GCPSubnet, 0, len(subnets))
			for _, subnetIface := range subnets {
				if subnet, ok := subnetIface.(map[string]interface{}); ok {
					gcpSubnet := GCPSubnet{}
					if name, ok := subnet["name"].(string); ok {
						gcpSubnet.Name = name
					}
					if cidr, ok := subnet["cidrBlock"].(string); ok {
						gcpSubnet.CIDR = cidr
					}
					if region, ok := subnet["region"].(string); ok {
						gcpSubnet.Region = region
					}
					if selfLink, ok := subnet["selfLink"].(string); ok {
						gcpSubnet.SelfLink = selfLink
					}
					if purpose, ok := subnet["purpose"].(string); ok {
						// Map purpose to role (e.g., "INTERNAL_HTTPS_LOAD_BALANCER" -> role)
						gcpSubnet.Role = purpose
					}
					detail.GCP.Subnets = append(detail.GCP.Subnets, gcpSubnet)
				}
			}
		}

		// Extract additional network components
		if additionalNetworkTags, ok := spec["additionalNetworkTags"].([]interface{}); ok && len(additionalNetworkTags) > 0 {
			// Network tags are typically used with firewall rules
			logger.Debug("found additional network tags", "count", len(additionalNetworkTags))
		}

		// Extract failureDomains (zones) if present
		if failureDomains, ok := spec["failureDomains"].([]interface{}); ok {
			logger.Debug("found failure domains", "count", len(failureDomains))
		}
	}

	// Extract status information
	if status, found, _ := unstructured.NestedMap(gcpCluster.Object, "status"); found {
		// Extract control plane endpoint
		if endpoint, ok := status["controlPlaneEndpoint"].(map[string]interface{}); ok {
			endpointInfo := &EndpointInfo{}
			if host, ok := endpoint["host"].(string); ok {
				endpointInfo.Host = host
			}
			if port, ok := endpoint["port"].(float64); ok {
				endpointInfo.Port = int32(port)
			} else if port, ok := endpoint["port"].(int64); ok {
				endpointInfo.Port = int32(port)
			}
			if endpointInfo.Host != "" {
				detail.ControlPlaneEndpoint = endpointInfo
			}
		}

		// Extract network information from status
		if network, ok := status["network"].(map[string]interface{}); ok {
			if selfLink, ok := network["selfLink"].(string); ok {
				if detail.GCP.Network == nil {
					detail.GCP.Network = &GCPNetwork{}
				}
				detail.GCP.Network.SelfLink = selfLink
			}
			if name, ok := network["name"].(string); ok {
				if detail.GCP.Network == nil {
					detail.GCP.Network = &GCPNetwork{}
				}
				detail.GCP.Network.Name = name
			}

			// Extract firewall rules from status
			if firewallRules, ok := network["firewallRules"].(map[string]interface{}); ok {
				for ruleName, ruleIface := range firewallRules {
					if rule, ok := ruleIface.(map[string]interface{}); ok {
						gcpRule := GCPFirewallRule{
							Name: ruleName,
						}
						if selfLink, ok := rule["selfLink"].(string); ok {
							gcpRule.SelfLink = selfLink
						}
						detail.GCP.FirewallRules = append(detail.GCP.FirewallRules, gcpRule)
					}
				}
			}

			// Extract router from status
			if router, ok := network["router"].(map[string]interface{}); ok {
				if name, ok := router["name"].(string); ok {
					gcpRouter := GCPRouter{
						Name: name,
					}
					if selfLink, ok := router["selfLink"].(string); ok {
						gcpRouter.SelfLink = selfLink
					}
					detail.GCP.Routers = append(detail.GCP.Routers, gcpRouter)
				}
			}
		}

		// Extract conditions using existing helper
		detail.Conditions = extractConditions(gcpCluster)

		// Extract ready status
		if ready, ok := status["ready"].(bool); ok {
			logger.Debug("GCPCluster ready status", "ready", ready)
		}
	}

	// Extract kubeconfig secret reference from ClusterDeployment status
	if kubeconfigSecret, found, _ := unstructured.NestedString(deployment.Object, "status", "kubeconfigSecretName"); found {
		detail.KubeconfigSecret = &ResourceReference{
			Name:      kubeconfigSecret,
			Namespace: namespace,
		}
	}

	logger.Info("GCP cluster detail fetched successfully",
		"name", name,
		"namespace", namespace,
		"project", detail.GCP.Project,
		"region", detail.GCP.Region,
	)

	return detail, nil
}
