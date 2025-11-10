package clusters

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func TestExtractServiceStatuses_MultipleServices(t *testing.T) {
	obj := &unstructured.Unstructured{
		Object: map[string]any{
			"status": map[string]any{
				"services": []any{
					map[string]any{
						"name":               "minio",
						"namespace":          "kcm-system",
						"template":           "minio-14-1-2",
						"state":              "Ready",
						"type":               "Helm",
						"version":            "14.1.2",
						"lastTransitionTime": "2025-11-10T10:00:00Z",
						"conditions": []any{
							map[string]any{
								"type":               "Helm",
								"status":             "True",
								"reason":             "InstallSucceeded",
								"message":            "Helm install succeeded",
								"lastTransitionTime": "2025-11-10T10:00:00Z",
							},
						},
					},
					map[string]any{
						"name":               "valkey",
						"template":           "valkey-0-1-0",
						"state":              "Ready",
						"type":               "Helm",
						"lastTransitionTime": "2025-11-10T10:01:00Z",
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

	services := ExtractServiceStatuses(obj)

	assert.Len(t, services, 3)

	// Check first service (minio)
	assert.Equal(t, "minio", services[0].Name)
	assert.Equal(t, "kcm-system", services[0].Namespace)
	assert.Equal(t, "minio-14-1-2", services[0].Template)
	assert.Equal(t, "Ready", services[0].State)
	assert.Equal(t, "Helm", services[0].Type)
	assert.Equal(t, "14.1.2", services[0].Version)
	assert.NotNil(t, services[0].LastTransitionTime)
	assert.Len(t, services[0].Conditions, 1)
	assert.Equal(t, "Helm", services[0].Conditions[0].Type)
	assert.Equal(t, "True", services[0].Conditions[0].Status)

	// Check second service (valkey)
	assert.Equal(t, "valkey", services[1].Name)
	assert.Equal(t, "valkey-0-1-0", services[1].Template)
	assert.Equal(t, "Ready", services[1].State)
	assert.NotNil(t, services[1].LastTransitionTime)

	// Check third service (cert-manager)
	assert.Equal(t, "cert-manager", services[2].Name)
	assert.Equal(t, "Pending", services[2].State)
	assert.Nil(t, services[2].LastTransitionTime)
}

func TestExtractServiceStatuses_EmptyServices(t *testing.T) {
	obj := &unstructured.Unstructured{
		Object: map[string]any{
			"status": map[string]any{
				"services": []any{},
			},
		},
	}

	services := ExtractServiceStatuses(obj)
	assert.Nil(t, services)
}

func TestExtractServiceStatuses_MissingStatusServices(t *testing.T) {
	obj := &unstructured.Unstructured{
		Object: map[string]any{
			"status": map[string]any{},
		},
	}

	services := ExtractServiceStatuses(obj)
	assert.Nil(t, services)
}

func TestExtractServiceStatuses_NilObject(t *testing.T) {
	services := ExtractServiceStatuses(nil)
	assert.Nil(t, services)
}

func TestExtractServiceStatuses_WithFailedConditions(t *testing.T) {
	obj := &unstructured.Unstructured{
		Object: map[string]any{
			"status": map[string]any{
				"services": []any{
					map[string]any{
						"name":     "grafana",
						"template": "grafana-10-1-0",
						"state":    "Failed",
						"type":     "Helm",
						"conditions": []any{
							map[string]any{
								"type":               "Helm",
								"status":             "False",
								"reason":             "InstallFailed",
								"message":            "Helm install failed: image pull timeout for grafana:10.1.0",
								"lastTransitionTime": "2025-11-10T14:40:15Z",
							},
						},
					},
				},
			},
		},
	}

	services := ExtractServiceStatuses(obj)

	assert.Len(t, services, 1)
	assert.Equal(t, "grafana", services[0].Name)
	assert.Equal(t, "Failed", services[0].State)
	assert.Len(t, services[0].Conditions, 1)
	assert.Equal(t, "False", services[0].Conditions[0].Status)
	assert.Equal(t, "InstallFailed", services[0].Conditions[0].Reason)
	assert.Equal(t, "Helm install failed: image pull timeout for grafana:10.1.0", services[0].Conditions[0].Message)

	expectedTime := time.Date(2025, 11, 10, 14, 40, 15, 0, time.UTC)
	assert.NotNil(t, services[0].Conditions[0].LastTransitionTime)
	assert.Equal(t, expectedTime, *services[0].Conditions[0].LastTransitionTime)
}

func TestExtractServiceStatuses_SkipsServicesWithoutName(t *testing.T) {
	obj := &unstructured.Unstructured{
		Object: map[string]any{
			"status": map[string]any{
				"services": []any{
					map[string]any{
						"template": "some-template",
						"state":    "Ready",
					},
					map[string]any{
						"name":     "valid-service",
						"template": "valid-template",
						"state":    "Ready",
					},
				},
			},
		},
	}

	services := ExtractServiceStatuses(obj)

	assert.Len(t, services, 1)
	assert.Equal(t, "valid-service", services[0].Name)
}

func TestExtractServiceStatuses_UpgradingState(t *testing.T) {
	obj := &unstructured.Unstructured{
		Object: map[string]any{
			"status": map[string]any{
				"services": []any{
					map[string]any{
						"name":               "minio",
						"template":           "minio-14-2-0",
						"state":              "Upgrading",
						"type":               "Helm",
						"version":            "14.2.0",
						"lastTransitionTime": "2025-11-10T15:00:00Z",
					},
				},
			},
		},
	}

	services := ExtractServiceStatuses(obj)

	assert.Len(t, services, 1)
	assert.Equal(t, "minio", services[0].Name)
	assert.Equal(t, "Upgrading", services[0].State)
	assert.Equal(t, "minio-14-2-0", services[0].Template)
	assert.Equal(t, "14.2.0", services[0].Version)
}
