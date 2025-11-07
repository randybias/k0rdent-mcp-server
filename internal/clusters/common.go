package clusters

import (
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// IsResourceReady checks if a Kubernetes resource has a Ready=True condition.
// This is a common status pattern used by k0rdent CRDs and CAPI resources.
func IsResourceReady(obj *unstructured.Unstructured) bool {
	conditions, found, err := unstructured.NestedSlice(obj.Object, "status", "conditions")
	if err != nil || !found {
		return false
	}

	for _, cond := range conditions {
		condMap, ok := cond.(map[string]interface{})
		if !ok {
			continue
		}

		condType, _ := condMap["type"].(string)
		condStatus, _ := condMap["status"].(string)

		if condType == "Ready" && condStatus == "True" {
			return true
		}
	}

	return false
}
