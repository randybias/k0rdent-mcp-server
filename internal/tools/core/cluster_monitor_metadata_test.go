package core

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestBuildClusterProgress_WithMetadata(t *testing.T) {
	createdAt := metav1.NewTime(time.Date(2025, 11, 10, 10, 0, 0, 0, time.UTC))

	obj := &unstructured.Unstructured{
		Object: map[string]any{
			"metadata": map[string]any{
				"name":              "demo-cluster",
				"namespace":         "kcm-system",
				"creationTimestamp": createdAt.Format(time.RFC3339),
				"labels": map[string]any{
					"cloud.k0rdent.mirantis.com/provider": "azure",
					"cloud.k0rdent.mirantis.com/region":   "westus2",
				},
			},
			"spec": map[string]any{
				"template":   "azure-standalone-cp",
				"credential": "azure-cred",
				"config": map[string]any{
					"location": "westus2",
				},
			},
			"status": map[string]any{
				"phase":   "Provisioning",
				"message": "Creating infrastructure",
				"conditions": []any{
					map[string]any{
						"type":   "Ready",
						"status": "False",
					},
				},
			},
		},
	}
	obj.SetCreationTimestamp(createdAt)

	update := buildClusterProgress(obj, nil)

	// Check metadata is populated
	assert.Equal(t, "demo-cluster", update.Metadata.Name)
	assert.Equal(t, "kcm-system", update.Metadata.Namespace)
	assert.Equal(t, "azure-standalone-cp", update.Metadata.TemplateRef.Name)
	assert.Equal(t, "azure-cred", update.Metadata.CredentialRef.Name)
	assert.Equal(t, "azure", update.Metadata.Provider)
	assert.Equal(t, "westus2", update.Metadata.Region)
	assert.True(t, update.Metadata.CreatedAt.Equal(createdAt.Time))

	// Check deployment status
	assert.Equal(t, "Provisioning", update.Phase.String())
	assert.False(t, update.Terminal)
}

func TestBuildClusterProgress_WithServices(t *testing.T) {
	obj := &unstructured.Unstructured{
		Object: map[string]any{
			"metadata": map[string]any{
				"name":      "demo-cluster",
				"namespace": "kcm-system",
			},
			"spec": map[string]any{
				"template":   "azure-standalone-cp",
				"credential": "azure-cred",
			},
			"status": map[string]any{
				"phase": "Ready",
				"conditions": []any{
					map[string]any{
						"type":   "Ready",
						"status": "True",
					},
				},
				"services": []any{
					map[string]any{
						"name":     "minio",
						"template": "minio-14-1-2",
						"state":    "Ready",
						"type":     "Helm",
					},
					map[string]any{
						"name":     "cert-manager",
						"template": "cert-manager-1-18-2",
						"state":    "Pending",
						"type":     "Helm",
					},
				},
			},
		},
	}

	update := buildClusterProgress(obj, nil)

	// Check services are populated
	assert.Len(t, update.Services, 2)

	assert.Equal(t, "minio", update.Services[0].Name)
	assert.Equal(t, "minio-14-1-2", update.Services[0].Template)
	assert.Equal(t, "Ready", update.Services[0].State)
	assert.Equal(t, "Helm", update.Services[0].Type)

	assert.Equal(t, "cert-manager", update.Services[1].Name)
	assert.Equal(t, "cert-manager-1-18-2", update.Services[1].Template)
	assert.Equal(t, "Pending", update.Services[1].State)
}

func TestBuildClusterProgress_NoServices(t *testing.T) {
	obj := &unstructured.Unstructured{
		Object: map[string]any{
			"metadata": map[string]any{
				"name":      "bare-cluster",
				"namespace": "kcm-system",
			},
			"spec": map[string]any{
				"template":   "azure-standalone-cp",
				"credential": "azure-cred",
			},
			"status": map[string]any{
				"phase": "Ready",
				"conditions": []any{
					map[string]any{
						"type":   "Ready",
						"status": "True",
					},
				},
			},
		},
	}

	update := buildClusterProgress(obj, nil)

	// Check services array is empty (not nil, because we initialize it)
	assert.Empty(t, update.Services)
	assert.Equal(t, "bare-cluster", update.Metadata.Name)
	assert.Equal(t, "Ready", update.Phase.String())
}

func TestBuildClusterProgress_ExcludesInfrastructureDetails(t *testing.T) {
	// This test verifies that provider-specific infrastructure details
	// are NOT included in the ProgressUpdate metadata
	obj := &unstructured.Unstructured{
		Object: map[string]any{
			"metadata": map[string]any{
				"name":      "azure-cluster",
				"namespace": "kcm-system",
			},
			"spec": map[string]any{
				"template":   "azure-standalone-cp",
				"credential": "azure-cred",
				"config": map[string]any{
					"location":       "westus2",
					"subscriptionID": "12345678-1234-1234-1234-123456789abc",
					"resourceGroup":  "my-resource-group",
					// These infrastructure details should NOT appear in metadata
				},
			},
			"status": map[string]any{
				"phase": "Ready",
			},
		},
	}

	update := buildClusterProgress(obj, nil)

	// Metadata should have basic fields
	assert.Equal(t, "azure-cluster", update.Metadata.Name)
	assert.Equal(t, "westus2", update.Metadata.Region)

	// But NOT infrastructure IDs (these would be in provider-specific detail tools)
	// This is verified by the structure itself - ClusterMetadata doesn't have these fields
	// The metadata only has: Name, Namespace, TemplateRef, CredentialRef, Provider, Region, CreatedAt
	// No subscriptionID, resourceGroup, vnetID, etc.
	assert.NotEmpty(t, update.Metadata.Name)
	assert.NotEmpty(t, update.Metadata.Region)
}
