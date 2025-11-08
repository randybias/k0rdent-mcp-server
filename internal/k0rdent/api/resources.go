package api

import (
	"context"
	"fmt"

	"github.com/k0rdent/mcp-k0rdent-server/internal/clusters"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
)

var (
	serviceTemplateGVR                  = schema.GroupVersionResource{Group: "k0rdent.mirantis.com", Version: "v1beta1", Resource: "servicetemplates"}
	clusterDeploymentGVR                = schema.GroupVersionResource{Group: "k0rdent.mirantis.com", Version: "v1beta1", Resource: "clusterdeployments"}
	multiClusterServiceGVR              = schema.GroupVersionResource{Group: "k0rdent.mirantis.com", Version: "v1beta1", Resource: "multiclusterservices"}
	runtimeDefaultUnstructuredConverter = runtime.DefaultUnstructuredConverter
)

// ServiceTemplateSummary provides a compact view of a ServiceTemplate.
type ServiceTemplateSummary struct {
	Name        string            `json:"name"`
	Namespace   string            `json:"namespace"`
	Labels      map[string]string `json:"labels,omitempty"`
	Version     string            `json:"version,omitempty"`
	ChartKind   string            `json:"chartKind,omitempty"`
	ChartName   string            `json:"chartName,omitempty"`
	Description string            `json:"description,omitempty"`
}

// ClusterDeploymentSummary provides a compact view of a ClusterDeployment.
type ClusterDeploymentSummary = clusters.ClusterDeploymentSummary

// MultiClusterServiceSummary provides a compact view of a MultiClusterService.
type MultiClusterServiceSummary struct {
	Name         string            `json:"name"`
	Namespace    string            `json:"namespace"`
	Labels       map[string]string `json:"labels,omitempty"`
	MatchLabels  map[string]string `json:"matchLabels,omitempty"`
	ServiceCount int               `json:"serviceCount"`
}

// ListServiceTemplates returns ServiceTemplate summaries across all namespaces.
func ListServiceTemplates(ctx context.Context, client dynamic.Interface) ([]ServiceTemplateSummary, error) {
	list, err := client.Resource(serviceTemplateGVR).Namespace(metav1.NamespaceAll).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("list service templates: %w", err)
	}
	summaries := make([]ServiceTemplateSummary, 0, len(list.Items))
	for i := range list.Items {
		summaries = append(summaries, SummarizeServiceTemplate(&list.Items[i]))
	}
	return summaries, nil
}

// ListClusterDeployments returns ClusterDeployment summaries filtered by an optional label selector.
func ListClusterDeployments(ctx context.Context, client dynamic.Interface, selector string) ([]ClusterDeploymentSummary, error) {
	var opts metav1.ListOptions
	if selector != "" {
		opts.LabelSelector = selector
	}
	list, err := client.Resource(clusterDeploymentGVR).Namespace(metav1.NamespaceAll).List(ctx, opts)
	if err != nil {
		return nil, fmt.Errorf("list cluster deployments: %w", err)
	}
	summaries := make([]ClusterDeploymentSummary, 0, len(list.Items))
	for i := range list.Items {
		summaries = append(summaries, SummarizeClusterDeployment(&list.Items[i]))
	}
	return summaries, nil
}

// ListMultiClusterServices returns MultiClusterService summaries filtered by an optional label selector.
func ListMultiClusterServices(ctx context.Context, client dynamic.Interface, selector string) ([]MultiClusterServiceSummary, error) {
	var opts metav1.ListOptions
	if selector != "" {
		opts.LabelSelector = selector
	}
	list, err := client.Resource(multiClusterServiceGVR).Namespace(metav1.NamespaceAll).List(ctx, opts)
	if err != nil {
		return nil, fmt.Errorf("list multi cluster services: %w", err)
	}
	summaries := make([]MultiClusterServiceSummary, 0, len(list.Items))
	for i := range list.Items {
		summaries = append(summaries, SummarizeMultiClusterService(&list.Items[i]))
	}
	return summaries, nil
}

func SummarizeServiceTemplate(obj *unstructured.Unstructured) ServiceTemplateSummary {
	if obj == nil {
		return ServiceTemplateSummary{}
	}
	version, _, _ := unstructured.NestedString(obj.Object, "spec", "version")
	chartKind, _, _ := unstructured.NestedString(obj.Object, "spec", "helm", "chartRef", "kind")
	chartName, _, _ := unstructured.NestedString(obj.Object, "spec", "helm", "chartRef", "name")
	description, _, _ := unstructured.NestedString(obj.Object, "spec", "helm", "chartSource", "description")
	if description == "" {
		description, _, _ = unstructured.NestedString(obj.Object, "spec", "resources", "description")
	}
	return ServiceTemplateSummary{
		Name:        obj.GetName(),
		Namespace:   obj.GetNamespace(),
		Labels:      obj.GetLabels(),
		Version:     version,
		ChartKind:   chartKind,
		ChartName:   chartName,
		Description: description,
	}
}

func SummarizeClusterDeployment(obj *unstructured.Unstructured) ClusterDeploymentSummary {
	return clusters.SummarizeClusterDeployment(obj)
}

func SummarizeMultiClusterService(obj *unstructured.Unstructured) MultiClusterServiceSummary {
	if obj == nil {
		return MultiClusterServiceSummary{}
	}
	matchLabels, _, _ := unstructured.NestedStringMap(obj.Object, "spec", "clusterSelector", "matchLabels")
	serviceCount := 0
	if list, found, _ := unstructured.NestedSlice(obj.Object, "spec", "serviceSpec", "services"); found {
		serviceCount = len(list)
	}
	return MultiClusterServiceSummary{
		Name:         obj.GetName(),
		Namespace:    obj.GetNamespace(),
		Labels:       obj.GetLabels(),
		MatchLabels:  matchLabels,
		ServiceCount: serviceCount,
	}
}

// MatchDeploymentSelector evaluates a label selector (matchLabels & matchExpressions) against the provided labels.
func MatchDeploymentSelector(objLabels map[string]string, selector map[string]any) bool {
	if objLabels == nil {
		objLabels = map[string]string{}
	}
	var lblSelector metav1.LabelSelector
	// Convert to LabelSelector via unstructured conversion.
	if selector != nil {
		err := runtimeDefaultUnstructuredConverter.FromUnstructured(selector, &lblSelector)
		if err != nil {
			// Fallback to matchLabels only.
			if matchLabels, ok := selector["matchLabels"].(map[string]any); ok {
				for k, v := range matchLabels {
					if val, ok := objLabels[k]; !ok || val != fmt.Sprintf("%v", v) {
						return false
					}
				}
				return true
			}
			return false
		}
	}
	k8sSelector, err := metav1.LabelSelectorAsSelector(&lblSelector)
	if err != nil {
		return false
	}
	return k8sSelector.Matches(labels.Set(objLabels))
}

// GroupVersionResources exposes the GVRs used by higher-level tooling.
func ServiceTemplateGVR() schema.GroupVersionResource     { return serviceTemplateGVR }
func ClusterDeploymentGVR() schema.GroupVersionResource   { return clusterDeploymentGVR }
func MultiClusterServiceGVR() schema.GroupVersionResource { return multiClusterServiceGVR }
