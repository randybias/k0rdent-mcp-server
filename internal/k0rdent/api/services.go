package api

import (
	"context"
	"errors"
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/dynamic"
)

const (
	defaultServiceFieldOwner = "mcp.services"
)

// ClusterServiceValuesFrom models a single valuesFrom entry for a managed service.
type ClusterServiceValuesFrom struct {
	Kind     string `json:"kind"`
	Name     string `json:"name"`
	Key      string `json:"key"`
	Optional *bool  `json:"optional,omitempty"`
}

// ClusterServiceHelmOptions captures Helm specific overrides for a managed service.
type ClusterServiceHelmOptions struct {
	Atomic        *bool  `json:"atomic,omitempty"`
	Wait          *bool  `json:"wait,omitempty"`
	Timeout       string `json:"timeout,omitempty"`
	CleanupOnFail *bool  `json:"cleanupOnFail,omitempty"`
	Description   string `json:"description,omitempty"`
	DisableHooks  *bool  `json:"disableHooks,omitempty"`
	Replace       *bool  `json:"replace,omitempty"`
	SkipCRDs      *bool  `json:"skipCRDs,omitempty"`
	MaxHistory    *int64 `json:"maxHistory,omitempty"`
}

// ClusterServiceApplySpec describes the service entry to create or update.
type ClusterServiceApplySpec struct {
	TemplateNamespace string                      `json:"templateNamespace"`
	TemplateName      string                      `json:"templateName"`
	ServiceName       string                      `json:"serviceName"`
	ServiceNamespace  *string                     `json:"serviceNamespace,omitempty"`
	Values            *string                     `json:"values,omitempty"`
	ValuesFrom        *[]ClusterServiceValuesFrom `json:"valuesFrom,omitempty"`
	HelmOptions       *ClusterServiceHelmOptions  `json:"helmOptions,omitempty"`
	DependsOn         *[]string                   `json:"dependsOn,omitempty"`
	Priority          *int64                      `json:"priority,omitempty"`
}

// ApplyClusterServiceOptions control how a service entry is merged into a ClusterDeployment.
type ApplyClusterServiceOptions struct {
	ClusterNamespace string                  `json:"clusterNamespace"`
	ClusterName      string                  `json:"clusterName"`
	FieldOwner       string                  `json:"fieldOwner"`
	DryRun           bool                    `json:"dryRun,omitempty"`
	Service          ClusterServiceApplySpec `json:"service"`
	ProviderConfig   *map[string]any         `json:"providerConfig,omitempty"`
}

// ApplyClusterServiceResult reports the outcome of a service apply operation.
type ApplyClusterServiceResult struct {
	Cluster *unstructured.Unstructured `json:"cluster"`
	Service map[string]any             `json:"service"`
}

// RemoveClusterServiceOptions specifies parameters for removing a service from a ClusterDeployment.
type RemoveClusterServiceOptions struct {
	ClusterNamespace string `json:"clusterNamespace"`
	ClusterName      string `json:"clusterName"`
	ServiceName      string `json:"serviceName"`
	FieldOwner       string `json:"fieldOwner"`
	DryRun           bool   `json:"dryRun,omitempty"`
}

// RemoveClusterServiceResult reports the outcome of a service removal.
type RemoveClusterServiceResult struct {
	RemovedService map[string]any             `json:"removedService"`
	UpdatedCluster *unstructured.Unstructured `json:"updatedCluster"`
	Message        string                     `json:"message"`
}

// ApplyClusterService fetches a ClusterDeployment, merges or creates the requested service entry,
// and applies the change via server-side apply. It returns the updated ClusterDeployment object
// (or the dry-run preview) plus the service payload that was sent.
func ApplyClusterService(ctx context.Context, client dynamic.Interface, opts ApplyClusterServiceOptions) (ApplyClusterServiceResult, error) {
	if client == nil {
		return ApplyClusterServiceResult{}, errors.New("dynamic client is required")
	}
	if opts.ClusterNamespace == "" {
		return ApplyClusterServiceResult{}, errors.New("cluster namespace is required")
	}
	if opts.ClusterName == "" {
		return ApplyClusterServiceResult{}, errors.New("cluster name is required")
	}
	if opts.Service.TemplateName == "" {
		return ApplyClusterServiceResult{}, errors.New("service template name is required")
	}

	serviceName := opts.Service.ServiceName
	if serviceName == "" {
		serviceName = opts.Service.TemplateName
	}
	if serviceName == "" {
		return ApplyClusterServiceResult{}, errors.New("service name could not be derived")
	}

	fieldOwner := opts.FieldOwner
	if fieldOwner == "" {
		fieldOwner = defaultServiceFieldOwner
	}

	cluster, err := client.
		Resource(clusterDeploymentGVR).
		Namespace(opts.ClusterNamespace).
		Get(ctx, opts.ClusterName, metav1.GetOptions{})
	if err != nil {
		return ApplyClusterServiceResult{}, fmt.Errorf("get cluster deployment: %w", err)
	}

	existingServices, err := existingServiceEntries(cluster)
	if err != nil {
		return ApplyClusterServiceResult{}, err
	}

	templateRef := buildTemplateReference(opts.Service.TemplateNamespace, opts.Service.TemplateName)

	updatedServices, appliedEntry := mergeServiceEntries(existingServices, serviceName, opts, templateRef)

	serviceSpec := map[string]any{
		"services": toInterfaceSlice(updatedServices),
	}

	if opts.ProviderConfig != nil {
		providerMap := existingProviderConfig(cluster)
		providerMap["config"] = deepCopyMap(*opts.ProviderConfig)
		serviceSpec["provider"] = providerMap
	}

	payload := &unstructured.Unstructured{
		Object: map[string]any{
			"apiVersion": cluster.GetAPIVersion(),
			"kind":       cluster.GetKind(),
			"metadata": map[string]any{
				"name":      opts.ClusterName,
				"namespace": opts.ClusterNamespace,
			},
			"spec": map[string]any{
				"serviceSpec": serviceSpec,
			},
		},
	}

	applyOptions := metav1.ApplyOptions{
		FieldManager: fieldOwner,
		Force:        true,
	}
	if opts.DryRun {
		applyOptions.DryRun = []string{metav1.DryRunAll}
	}

	result, err := client.
		Resource(clusterDeploymentGVR).
		Namespace(opts.ClusterNamespace).
		Apply(ctx, opts.ClusterName, payload, applyOptions)
	if err != nil {
		return ApplyClusterServiceResult{}, fmt.Errorf("apply cluster service: %w", err)
	}

	return ApplyClusterServiceResult{
		Cluster: result,
		Service: deepCopyMap(appliedEntry),
	}, nil
}

func existingServiceEntries(cluster *unstructured.Unstructured) ([]map[string]any, error) {
	if cluster == nil {
		return nil, errors.New("cluster object is nil")
	}
	spec, _ := cluster.Object["spec"].(map[string]any)
	serviceSpec, _ := spec["serviceSpec"].(map[string]any)
	raw, _ := serviceSpec["services"].([]any)
	entries := make([]map[string]any, 0, len(raw))
	for _, entry := range raw {
		m, ok := entry.(map[string]any)
		if !ok {
			continue
		}
		entries = append(entries, deepCopyMap(m))
	}
	return entries, nil
}

func mergeServiceEntries(existing []map[string]any, targetName string, opts ApplyClusterServiceOptions, templateRef string) ([]map[string]any, map[string]any) {
	resolvedName := targetName
	var applied map[string]any
	updated := make([]map[string]any, 0, len(existing))
	for _, entry := range existing {
		name, _ := entry["name"].(string)
		if name == resolvedName {
			applied = applyServiceMutations(entry, opts.Service, opts.ClusterNamespace, templateRef)
			updated = append(updated, applied)
		} else {
			updated = append(updated, entry)
		}
	}
	if applied == nil {
		applied = applyServiceMutations(nil, opts.Service, opts.ClusterNamespace, templateRef)
		updated = append(updated, applied)
	}
	return updated, applied
}

func applyServiceMutations(existing map[string]any, spec ClusterServiceApplySpec, fallbackNamespace string, templateRef string) map[string]any {
	dest := deepCopyMap(existing)
	if dest == nil {
		dest = make(map[string]any)
	}

	serviceName := spec.ServiceName
	if serviceName == "" {
		serviceName = spec.TemplateName
	}
	dest["name"] = serviceName

	namespace := fallbackNamespace
	if spec.ServiceNamespace != nil && *spec.ServiceNamespace != "" {
		namespace = *spec.ServiceNamespace
	} else if existingNS, ok := dest["namespace"].(string); ok && existingNS != "" {
		namespace = existingNS
	}
	dest["namespace"] = namespace
	dest["template"] = templateRef

	if spec.Values != nil {
		dest["values"] = *spec.Values
	} else {
		delete(dest, "values")
	}
	if spec.ValuesFrom != nil {
		dest["valuesFrom"] = valuesFromToSlice(*spec.ValuesFrom)
	}
	if spec.HelmOptions != nil {
		if helmMap := spec.HelmOptions.toMap(); len(helmMap) > 0 {
			dest["helmOptions"] = helmMap
		} else {
			delete(dest, "helmOptions")
		}
	}
	if spec.DependsOn != nil {
		dest["dependsOn"] = stringSliceToInterface(*spec.DependsOn)
	}
	if spec.Priority != nil {
		dest["priority"] = *spec.Priority
	}
	return dest
}

func buildTemplateReference(namespace, name string) string {
	return name
}

func valuesFromToSlice(list []ClusterServiceValuesFrom) []any {
	if len(list) == 0 {
		return []any{}
	}
	result := make([]any, 0, len(list))
	for _, item := range list {
		entry := map[string]any{
			"kind": item.Kind,
			"name": item.Name,
			"key":  item.Key,
		}
		if item.Optional != nil {
			entry["optional"] = *item.Optional
		}
		result = append(result, entry)
	}
	return result
}

func (h *ClusterServiceHelmOptions) toMap() map[string]any {
	if h == nil {
		return nil
	}
	result := map[string]any{}
	if h.Atomic != nil {
		result["atomic"] = *h.Atomic
	}
	if h.Wait != nil {
		result["wait"] = *h.Wait
	}
	if h.Timeout != "" {
		result["timeout"] = h.Timeout
	}
	if h.CleanupOnFail != nil {
		result["cleanupOnFail"] = *h.CleanupOnFail
	}
	if h.Description != "" {
		result["description"] = h.Description
	}
	if h.DisableHooks != nil {
		result["disableHooks"] = *h.DisableHooks
	}
	if h.Replace != nil {
		result["replace"] = *h.Replace
	}
	if h.SkipCRDs != nil {
		result["skipCRDs"] = *h.SkipCRDs
	}
	if h.MaxHistory != nil {
		result["maxHistory"] = *h.MaxHistory
	}
	return result
}

func stringSliceToInterface(in []string) []any {
	if len(in) == 0 {
		return []any{}
	}
	out := make([]any, len(in))
	for i, v := range in {
		out[i] = v
	}
	return out
}

func toInterfaceSlice(in []map[string]any) []any {
	out := make([]any, len(in))
	for i, entry := range in {
		out[i] = entry
	}
	return out
}

func deepCopyMap(in map[string]any) map[string]any {
	if in == nil {
		return nil
	}
	out := make(map[string]any, len(in))
	for k, v := range in {
		out[k] = deepCopyValue(v)
	}
	return out
}

func deepCopyValue(val any) any {
	switch v := val.(type) {
	case map[string]any:
		return deepCopyMap(v)
	case []any:
		cp := make([]any, len(v))
		for i, item := range v {
			cp[i] = deepCopyValue(item)
		}
		return cp
	default:
		return v
	}
}

func existingProviderConfig(cluster *unstructured.Unstructured) map[string]any {
	if cluster == nil {
		return map[string]any{}
	}
	provider, found, err := unstructured.NestedMap(cluster.Object, "spec", "serviceSpec", "provider")
	if err != nil || !found || provider == nil {
		return map[string]any{}
	}
	return deepCopyMap(provider)
}

// filterServiceEntries removes a service entry from the services slice by name.
// Returns the filtered slice and the removed entry (nil if not found).
func filterServiceEntries(existing []map[string]any, targetName string) ([]map[string]any, map[string]any) {
	if len(existing) == 0 {
		return existing, nil
	}

	var removed map[string]any
	filtered := make([]map[string]any, 0, len(existing))

	for _, entry := range existing {
		name, _ := entry["name"].(string)
		if name == targetName {
			removed = deepCopyMap(entry)
		} else {
			filtered = append(filtered, entry)
		}
	}

	return filtered, removed
}

// RemoveClusterService removes a service entry from ClusterDeployment.spec.serviceSpec.services[]
// and applies the change via server-side apply. It returns the removed service entry (if found),
// the updated ClusterDeployment object (or the dry-run preview), and a status message.
func RemoveClusterService(ctx context.Context, client dynamic.Interface, opts RemoveClusterServiceOptions) (RemoveClusterServiceResult, error) {
	if client == nil {
		return RemoveClusterServiceResult{}, errors.New("dynamic client is required")
	}
	if opts.ClusterNamespace == "" {
		return RemoveClusterServiceResult{}, errors.New("cluster namespace is required")
	}
	if opts.ClusterName == "" {
		return RemoveClusterServiceResult{}, errors.New("cluster name is required")
	}
	if opts.ServiceName == "" {
		return RemoveClusterServiceResult{}, errors.New("service name is required")
	}

	fieldOwner := opts.FieldOwner
	if fieldOwner == "" {
		fieldOwner = defaultServiceFieldOwner
	}

	cluster, err := client.
		Resource(clusterDeploymentGVR).
		Namespace(opts.ClusterNamespace).
		Get(ctx, opts.ClusterName, metav1.GetOptions{})
	if err != nil {
		return RemoveClusterServiceResult{}, fmt.Errorf("get cluster deployment: %w", err)
	}

	existingServices, err := existingServiceEntries(cluster)
	if err != nil {
		return RemoveClusterServiceResult{}, err
	}

	filteredServices, removedEntry := filterServiceEntries(existingServices, opts.ServiceName)

	if removedEntry == nil {
		return RemoveClusterServiceResult{
			RemovedService: nil,
			UpdatedCluster: cluster,
			Message:        "service not found (already removed)",
		}, nil
	}

	payload := &unstructured.Unstructured{
		Object: map[string]any{
			"apiVersion": cluster.GetAPIVersion(),
			"kind":       cluster.GetKind(),
			"metadata": map[string]any{
				"name":      opts.ClusterName,
				"namespace": opts.ClusterNamespace,
			},
			"spec": map[string]any{
				"serviceSpec": map[string]any{
					"services": toInterfaceSlice(filteredServices),
				},
			},
		},
	}

	applyOptions := metav1.ApplyOptions{
		FieldManager: fieldOwner,
		Force:        true,
	}
	if opts.DryRun {
		applyOptions.DryRun = []string{metav1.DryRunAll}
	}

	result, err := client.
		Resource(clusterDeploymentGVR).
		Namespace(opts.ClusterNamespace).
		Apply(ctx, opts.ClusterName, payload, applyOptions)
	if err != nil {
		return RemoveClusterServiceResult{}, fmt.Errorf("apply cluster service removal: %w", err)
	}

	return RemoveClusterServiceResult{
		RemovedService: removedEntry,
		UpdatedCluster: result,
		Message:        "service removed successfully",
	}, nil
}
