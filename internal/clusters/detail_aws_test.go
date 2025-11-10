package clusters

import (
	"context"
	"log/slog"
	"regexp"
	"testing"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/dynamic/fake"
)

// TestGetAWSClusterDetail_Success tests successful AWS cluster detail retrieval with complete data
func TestGetAWSClusterDetail_Success(t *testing.T) {
	// Create a ClusterDeployment with AWS configuration
	cd := createTestClusterDeployment("test-aws-cluster", "kcm-system", map[string]string{
		"k0rdent.mirantis.com/provider": "aws",
	})
	unstructured.SetNestedField(cd.Object, "aws-template", "spec", "template")
	unstructured.SetNestedField(cd.Object, "aws-credential", "spec", "credential")
	unstructured.SetNestedField(cd.Object, "us-west-2", "spec", "config", "region")
	unstructured.SetNestedField(cd.Object, "kubeconfig-secret", "status", "kubeconfigSecret")

	// Create an AWSCluster CR with complete infrastructure details
	awsCluster := createTestAWSCluster("test-aws-cluster", "kcm-system", map[string]string{})

	scheme := runtime.NewScheme()
	client := fake.NewSimpleDynamicClient(scheme, cd, awsCluster)

	manager := &Manager{
		dynamicClient:   client,
		globalNamespace: "kcm-system",
		logger:          slog.Default(),
	}

	detail, err := manager.GetAWSClusterDetail(context.Background(), "kcm-system", "test-aws-cluster")
	if err != nil {
		t.Fatalf("GetAWSClusterDetail returned error: %v", err)
	}

	// Verify basic metadata
	if detail.Name != "test-aws-cluster" {
		t.Errorf("expected name %q, got %q", "test-aws-cluster", detail.Name)
	}
	if detail.Namespace != "kcm-system" {
		t.Errorf("expected namespace %q, got %q", "kcm-system", detail.Namespace)
	}
	if detail.Provider != "aws" {
		t.Errorf("expected provider %q, got %q", "aws", detail.Provider)
	}
	if detail.Region != "us-west-2" {
		t.Errorf("expected region %q, got %q", "us-west-2", detail.Region)
	}

	// Verify template reference
	if detail.TemplateRef.Name != "aws-template" {
		t.Errorf("expected template ref name %q, got %q", "aws-template", detail.TemplateRef.Name)
	}

	// Verify credential reference
	if detail.CredentialRef.Name != "aws-credential" {
		t.Errorf("expected credential ref name %q, got %q", "aws-credential", detail.CredentialRef.Name)
	}

	// Verify kubeconfig secret
	if detail.KubeconfigSecret == nil {
		t.Error("expected kubeconfig secret reference, got nil")
	} else if detail.KubeconfigSecret.Name != "kubeconfig-secret" {
		t.Errorf("expected kubeconfig secret name %q, got %q", "kubeconfig-secret", detail.KubeconfigSecret.Name)
	}

	// Verify AWS infrastructure details
	if detail.AWS.Region != "us-east-1" {
		t.Errorf("expected AWS region %q, got %q", "us-east-1", detail.AWS.Region)
	}
	if detail.AWS.AccountID != "123456789012" {
		t.Errorf("expected AWS account ID %q, got %q", "123456789012", detail.AWS.AccountID)
	}

	// Verify VPC
	if detail.AWS.VPC == nil {
		t.Error("expected VPC, got nil")
	} else {
		if detail.AWS.VPC.ID != "vpc-12345678" {
			t.Errorf("expected VPC ID %q, got %q", "vpc-12345678", detail.AWS.VPC.ID)
		}
		if detail.AWS.VPC.CIDR != "10.0.0.0/16" {
			t.Errorf("expected VPC CIDR %q, got %q", "10.0.0.0/16", detail.AWS.VPC.CIDR)
		}
	}

	// Verify subnets
	if len(detail.AWS.Subnets) != 2 {
		t.Errorf("expected 2 subnets, got %d", len(detail.AWS.Subnets))
	} else {
		subnet := detail.AWS.Subnets[0]
		if subnet.ID != "subnet-11111111" {
			t.Errorf("expected subnet ID %q, got %q", "subnet-11111111", subnet.ID)
		}
		if subnet.CIDR != "10.0.1.0/24" {
			t.Errorf("expected subnet CIDR %q, got %q", "10.0.1.0/24", subnet.CIDR)
		}
		if subnet.AvailabilityZone != "us-east-1a" {
			t.Errorf("expected AZ %q, got %q", "us-east-1a", subnet.AvailabilityZone)
		}
		if !subnet.IsPublic {
			t.Error("expected subnet to be public")
		}
	}

	// Verify internet gateway
	if detail.AWS.InternetGateway == nil {
		t.Error("expected internet gateway, got nil")
	} else if detail.AWS.InternetGateway.ID != "igw-12345678" {
		t.Errorf("expected IGW ID %q, got %q", "igw-12345678", detail.AWS.InternetGateway.ID)
	}

	// Verify NAT gateways
	if len(detail.AWS.NATGateways) != 1 {
		t.Errorf("expected 1 NAT gateway, got %d", len(detail.AWS.NATGateways))
	} else {
		nat := detail.AWS.NATGateways[0]
		if nat.ID != "nat-12345678" {
			t.Errorf("expected NAT gateway ID %q, got %q", "nat-12345678", nat.ID)
		}
		if nat.SubnetID != "subnet-11111111" {
			t.Errorf("expected NAT gateway subnet ID %q, got %q", "subnet-11111111", nat.SubnetID)
		}
	}

	// Verify load balancers
	if len(detail.AWS.LoadBalancers) != 1 {
		t.Errorf("expected 1 load balancer, got %d", len(detail.AWS.LoadBalancers))
	} else {
		lb := detail.AWS.LoadBalancers[0]
		if lb.DNSName != "test-lb-123456.us-east-1.elb.amazonaws.com" {
			t.Errorf("expected LB DNS name %q, got %q", "test-lb-123456.us-east-1.elb.amazonaws.com", lb.DNSName)
		}
		if lb.Type != "classic" {
			t.Errorf("expected LB type %q, got %q", "classic", lb.Type)
		}
	}

	// Verify security groups
	if len(detail.AWS.SecurityGroups) != 2 {
		t.Errorf("expected 2 security groups, got %d", len(detail.AWS.SecurityGroups))
	}

	// Verify IAM roles
	if len(detail.AWS.IAMRoles) != 2 {
		t.Errorf("expected 2 IAM roles, got %d", len(detail.AWS.IAMRoles))
	}

	// Verify control plane endpoint
	if detail.ControlPlaneEndpoint == nil {
		t.Error("expected control plane endpoint, got nil")
	} else {
		if detail.ControlPlaneEndpoint.Host != "test-cluster.us-east-1.elb.amazonaws.com" {
			t.Errorf("expected endpoint host %q, got %q", "test-cluster.us-east-1.elb.amazonaws.com", detail.ControlPlaneEndpoint.Host)
		}
		if detail.ControlPlaneEndpoint.Port != 6443 {
			t.Errorf("expected endpoint port %d, got %d", 6443, detail.ControlPlaneEndpoint.Port)
		}
	}
}

// TestGetAWSClusterDetail_MinimalData tests successful detail retrieval with minimal optional fields
func TestGetAWSClusterDetail_MinimalData(t *testing.T) {
	// Create a minimal ClusterDeployment
	cd := createTestClusterDeployment("minimal-cluster", "kcm-system", nil)
	unstructured.SetNestedField(cd.Object, "aws-template", "spec", "template")
	unstructured.SetNestedField(cd.Object, "aws-credential", "spec", "credential")

	// Create a minimal AWSCluster CR
	// Note: Must include at least VPC to avoid nil pointer in logging (line 104 of detail_aws.go)
	awsCluster := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "infrastructure.cluster.x-k8s.io/v1beta2",
			"kind":       "AWSCluster",
			"metadata": map[string]interface{}{
				"name":      "minimal-cluster",
				"namespace": "kcm-system",
			},
			"spec": map[string]interface{}{
				"region": "us-west-2",
				"network": map[string]interface{}{
					"vpc": map[string]interface{}{
						"id": "vpc-minimal",
					},
				},
			},
		},
	}

	scheme := runtime.NewScheme()
	client := fake.NewSimpleDynamicClient(scheme, cd, awsCluster)

	manager := &Manager{
		dynamicClient:   client,
		globalNamespace: "kcm-system",
		logger:          slog.Default(),
	}

	detail, err := manager.GetAWSClusterDetail(context.Background(), "kcm-system", "minimal-cluster")
	if err != nil {
		t.Fatalf("GetAWSClusterDetail returned error: %v", err)
	}

	// Verify basic metadata is present
	if detail.Name != "minimal-cluster" {
		t.Errorf("expected name %q, got %q", "minimal-cluster", detail.Name)
	}
	if detail.Namespace != "kcm-system" {
		t.Errorf("expected namespace %q, got %q", "kcm-system", detail.Namespace)
	}

	// Verify AWS infrastructure has minimal data
	if detail.AWS.Region != "us-west-2" {
		t.Errorf("expected AWS region %q, got %q", "us-west-2", detail.AWS.Region)
	}

	// Verify VPC exists (required to avoid nil pointer in logging)
	if detail.AWS.VPC == nil {
		t.Error("expected VPC, got nil")
	} else if detail.AWS.VPC.ID != "vpc-minimal" {
		t.Errorf("expected VPC ID %q, got %q", "vpc-minimal", detail.AWS.VPC.ID)
	}

	// Verify optional fields are empty
	if len(detail.AWS.Subnets) != 0 {
		t.Errorf("expected no subnets, got %d", len(detail.AWS.Subnets))
	}
	if detail.AWS.InternetGateway != nil {
		t.Error("expected no internet gateway, got non-nil")
	}
	if detail.KubeconfigSecret != nil {
		t.Error("expected no kubeconfig secret, got non-nil")
	}
	if detail.ControlPlaneEndpoint != nil {
		t.Error("expected no control plane endpoint, got non-nil")
	}
}

// TestGetAWSClusterDetail_ClusterDeploymentNotFound tests error when ClusterDeployment not found
func TestGetAWSClusterDetail_ClusterDeploymentNotFound(t *testing.T) {
	scheme := runtime.NewScheme()
	client := fake.NewSimpleDynamicClient(scheme)

	manager := &Manager{
		dynamicClient:   client,
		globalNamespace: "kcm-system",
		logger:          slog.Default(),
	}

	_, err := manager.GetAWSClusterDetail(context.Background(), "kcm-system", "nonexistent-cluster")
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	expectedMsg := "cluster deployment nonexistent-cluster not found in namespace kcm-system"
	if err.Error() != expectedMsg {
		t.Errorf("expected error message %q, got %q", expectedMsg, err.Error())
	}
}

// TestGetAWSClusterDetail_AWSClusterNotFound tests error when AWSCluster CR not found
func TestGetAWSClusterDetail_AWSClusterNotFound(t *testing.T) {
	// Create ClusterDeployment but no corresponding AWSCluster
	cd := createTestClusterDeployment("test-cluster", "kcm-system", nil)
	unstructured.SetNestedField(cd.Object, "aws-template", "spec", "template")
	unstructured.SetNestedField(cd.Object, "aws-credential", "spec", "credential")

	scheme := runtime.NewScheme()
	client := fake.NewSimpleDynamicClient(scheme, cd)

	manager := &Manager{
		dynamicClient:   client,
		globalNamespace: "kcm-system",
		logger:          slog.Default(),
	}

	_, err := manager.GetAWSClusterDetail(context.Background(), "kcm-system", "test-cluster")
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	expectedMsg := "AWSCluster CR test-cluster not found in namespace kcm-system (cluster may not be AWS or not yet provisioned)"
	if err.Error() != expectedMsg {
		t.Errorf("expected error message %q, got %q", expectedMsg, err.Error())
	}
}

// TestGetAWSClusterDetail_NamespaceValidation tests namespace validation with NamespaceFilter
func TestGetAWSClusterDetail_NamespaceValidation(t *testing.T) {
	tests := []struct {
		name            string
		namespaceFilter *regexp.Regexp
		targetNamespace string
		expectError     bool
	}{
		{
			name:            "no filter allows any namespace",
			namespaceFilter: nil,
			targetNamespace: "kcm-system",
			expectError:     false,
		},
		{
			name:            "filter matches namespace",
			namespaceFilter: regexp.MustCompile("^team-"),
			targetNamespace: "team-alpha",
			expectError:     false,
		},
		{
			name:            "global namespace always allowed with permissive filter",
			namespaceFilter: regexp.MustCompile(".*"),
			targetNamespace: "kcm-system",
			expectError:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create resources in target namespace
			cd := createTestClusterDeployment("test-cluster", tt.targetNamespace, nil)
			unstructured.SetNestedField(cd.Object, "aws-template", "spec", "template")
			unstructured.SetNestedField(cd.Object, "aws-credential", "spec", "credential")

			awsCluster := createTestAWSCluster("test-cluster", tt.targetNamespace, nil)

			scheme := runtime.NewScheme()
			client := fake.NewSimpleDynamicClient(scheme, cd, awsCluster)

			manager := &Manager{
				dynamicClient:   client,
				globalNamespace: "kcm-system",
				logger:          slog.Default(),
				namespaceFilter: tt.namespaceFilter,
			}

			detail, err := manager.GetAWSClusterDetail(context.Background(), tt.targetNamespace, "test-cluster")

			if tt.expectError {
				if err == nil {
					t.Error("expected error, got nil")
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
				if detail.Namespace != tt.targetNamespace {
					t.Errorf("expected namespace %q, got %q", tt.targetNamespace, detail.Namespace)
				}
			}
		})
	}
}

// TestGetAWSClusterDetail_TableDriven tests multiple scenarios using table-driven approach
func TestGetAWSClusterDetail_TableDriven(t *testing.T) {
	tests := []struct {
		name          string
		setupFunc     func() (cd, awsCluster *unstructured.Unstructured)
		namespace     string
		clusterName   string
		expectError   bool
		validateFunc  func(*testing.T, AWSClusterDetail)
	}{
		{
			name: "cluster with VPC and subnets",
			setupFunc: func() (cd, awsCluster *unstructured.Unstructured) {
				cd = createTestClusterDeployment("vpc-cluster", "kcm-system", nil)
				unstructured.SetNestedField(cd.Object, "aws-template", "spec", "template")
				unstructured.SetNestedField(cd.Object, "aws-credential", "spec", "credential")

				awsCluster = &unstructured.Unstructured{
					Object: map[string]interface{}{
						"apiVersion": "infrastructure.cluster.x-k8s.io/v1beta2",
						"kind":       "AWSCluster",
						"metadata": map[string]interface{}{
							"name":      "vpc-cluster",
							"namespace": "kcm-system",
						},
						"spec": map[string]interface{}{
							"region": "us-west-2",
							"network": map[string]interface{}{
								"vpc": map[string]interface{}{
									"id":        "vpc-abcdef",
									"cidrBlock": "10.100.0.0/16",
								},
								"subnets": []interface{}{
									map[string]interface{}{
										"resourceID":       "subnet-123",
										"cidrBlock":        "10.100.1.0/24",
										"availabilityZone": "us-west-2a",
										"isPublic":         true,
									},
								},
							},
						},
					},
				}
				return cd, awsCluster
			},
			namespace:   "kcm-system",
			clusterName: "vpc-cluster",
			expectError: false,
			validateFunc: func(t *testing.T, detail AWSClusterDetail) {
				if detail.AWS.VPC == nil {
					t.Error("expected VPC, got nil")
					return
				}
				if detail.AWS.VPC.ID != "vpc-abcdef" {
					t.Errorf("expected VPC ID %q, got %q", "vpc-abcdef", detail.AWS.VPC.ID)
				}
				if len(detail.AWS.Subnets) != 1 {
					t.Errorf("expected 1 subnet, got %d", len(detail.AWS.Subnets))
				}
			},
		},
		{
			name: "cluster with load balancer",
			setupFunc: func() (cd, awsCluster *unstructured.Unstructured) {
				cd = createTestClusterDeployment("lb-cluster", "kcm-system", nil)
				unstructured.SetNestedField(cd.Object, "aws-template", "spec", "template")
				unstructured.SetNestedField(cd.Object, "aws-credential", "spec", "credential")

				awsCluster = &unstructured.Unstructured{
					Object: map[string]interface{}{
						"apiVersion": "infrastructure.cluster.x-k8s.io/v1beta2",
						"kind":       "AWSCluster",
						"metadata": map[string]interface{}{
							"name":      "lb-cluster",
							"namespace": "kcm-system",
						},
						"spec": map[string]interface{}{
							"region": "us-east-1",
							"network": map[string]interface{}{
								"vpc": map[string]interface{}{
									"id": "vpc-lb-test",
								},
							},
							"controlPlaneLoadBalancer": map[string]interface{}{
								"scheme": "internet-facing",
								"name":   "test-lb",
							},
						},
						"status": map[string]interface{}{
							"network": map[string]interface{}{
								"apiServerElb": map[string]interface{}{
									"dnsName": "test-lb.us-east-1.elb.amazonaws.com",
								},
							},
						},
					},
				}
				return cd, awsCluster
			},
			namespace:   "kcm-system",
			clusterName: "lb-cluster",
			expectError: false,
			validateFunc: func(t *testing.T, detail AWSClusterDetail) {
				if len(detail.AWS.LoadBalancers) != 1 {
					t.Errorf("expected 1 load balancer, got %d", len(detail.AWS.LoadBalancers))
					return
				}
				lb := detail.AWS.LoadBalancers[0]
				if lb.DNSName != "test-lb.us-east-1.elb.amazonaws.com" {
					t.Errorf("expected LB DNS %q, got %q", "test-lb.us-east-1.elb.amazonaws.com", lb.DNSName)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cd, awsCluster := tt.setupFunc()

			scheme := runtime.NewScheme()
			client := fake.NewSimpleDynamicClient(scheme, cd, awsCluster)

			manager := &Manager{
				dynamicClient:   client,
				globalNamespace: "kcm-system",
				logger:          slog.Default(),
			}

			detail, err := manager.GetAWSClusterDetail(context.Background(), tt.namespace, tt.clusterName)

			if tt.expectError {
				if err == nil {
					t.Error("expected error, got nil")
				}
			} else {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				if tt.validateFunc != nil {
					tt.validateFunc(t, detail)
				}
			}
		})
	}
}

// createTestAWSCluster creates a test AWSCluster CR with complete infrastructure details
func createTestAWSCluster(name, namespace string, labels map[string]string) *unstructured.Unstructured {
	awsCluster := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "infrastructure.cluster.x-k8s.io/v1beta2",
			"kind":       "AWSCluster",
			"metadata": map[string]interface{}{
				"name":              name,
				"namespace":         namespace,
				"creationTimestamp": "2025-01-01T00:00:00Z",
			},
			"spec": map[string]interface{}{
				"region": "us-east-1",
				"identityRef": map[string]interface{}{
					"accountID": "123456789012",
				},
				"network": map[string]interface{}{
					"vpc": map[string]interface{}{
						"id":        "vpc-12345678",
						"cidrBlock": "10.0.0.0/16",
					},
					"subnets": []interface{}{
						map[string]interface{}{
							"resourceID":       "subnet-11111111",
							"cidrBlock":        "10.0.1.0/24",
							"availabilityZone": "us-east-1a",
							"isPublic":         true,
							"tags": map[string]interface{}{
								"kubernetes.io/role": "control-plane",
							},
						},
						map[string]interface{}{
							"resourceID":       "subnet-22222222",
							"cidrBlock":        "10.0.2.0/24",
							"availabilityZone": "us-east-1b",
							"isPublic":         false,
						},
					},
					"internetGatewayId": "igw-12345678",
					"natGateways": []interface{}{
						map[string]interface{}{
							"natGatewayId":     "nat-12345678",
							"subnetID":         "subnet-11111111",
							"availabilityZone": "us-east-1a",
						},
					},
					"securityGroups": map[string]interface{}{
						"controlplane": map[string]interface{}{
							"id": "sg-11111111",
						},
						"node": map[string]interface{}{
							"id": "sg-22222222",
						},
					},
				},
				"controlPlaneLoadBalancer": map[string]interface{}{
					"scheme": "internet-facing",
					"name":   "test-lb",
				},
				"controlPlaneEndpoint": map[string]interface{}{
					"host": "test-cluster.us-east-1.elb.amazonaws.com",
					"port": float64(6443),
				},
				"controlPlaneIAMInstanceProfile": "arn:aws:iam::123456789012:instance-profile/control-plane-profile",
				"nodeIAMInstanceProfile":         "arn:aws:iam::123456789012:instance-profile/worker-profile",
			},
			"status": map[string]interface{}{
				"ready": true,
				"network": map[string]interface{}{
					"apiServerElb": map[string]interface{}{
						"dnsName": "test-lb-123456.us-east-1.elb.amazonaws.com",
					},
					"securityGroups": map[string]interface{}{
						"controlplane": map[string]interface{}{
							"id": "sg-11111111",
						},
						"node": map[string]interface{}{
							"id": "sg-22222222",
						},
					},
				},
				"conditions": []interface{}{
					map[string]interface{}{
						"type":               "Ready",
						"status":             "True",
						"reason":             "InfrastructureReady",
						"lastTransitionTime": "2025-01-01T00:10:00Z",
					},
				},
			},
		},
	}

	if labels != nil {
		awsCluster.SetLabels(labels)
	}

	return awsCluster
}
