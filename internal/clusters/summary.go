package clusters

import (
	"strings"
	"time"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

const (
	labelCloudProvider   = "cloud.k0rdent.mirantis.com/provider"
	labelLegacyProvider  = "k0rdent.mirantis.com/provider"
	labelCloudRegion     = "cloud.k0rdent.mirantis.com/region"
	labelTemplateVersion = "k0rdent.mirantis.com/template-version"
	annotationOwner      = "k0rdent.mirantis.com/owner"
	annotationMgmtURL    = "k0rdent.mirantis.com/management-url"
	annotationCloudURL   = "cloud.k0rdent.mirantis.com/management-url"
)

// SummarizeClusterDeployment extracts a comprehensive summary from a ClusterDeployment.
func SummarizeClusterDeployment(obj *unstructured.Unstructured) ClusterDeploymentSummary {
	if obj == nil {
		return ClusterDeploymentSummary{}
	}

	summary := ClusterDeploymentSummary{
		Name:      obj.GetName(),
		Namespace: obj.GetNamespace(),
		Labels:    obj.GetLabels(),
		Owner:     extractOwner(obj),
		CreatedAt: obj.GetCreationTimestamp().Time,
	}

	if !summary.CreatedAt.IsZero() {
		age := time.Since(summary.CreatedAt).Seconds()
		if age < 0 {
			age = 0
		}
		summary.AgeSeconds = int64(age)
	}

	summary.TemplateRef = buildTemplateReference(obj, summary.Namespace)
	summary.CredentialRef = buildReferenceFromPath(obj, summary.Namespace, "spec", "credential")
	summary.ClusterIdentityRef = buildClusterIdentityReference(obj, summary.Namespace)
	summary.KubeconfigSecret = buildReferenceFromPath(obj, summary.Namespace, "status", "kubeconfigSecret")
	if summary.KubeconfigSecret.Name == "" {
		summary.KubeconfigSecret = buildReferenceFromPath(obj, summary.Namespace, "spec", "kubeconfigSecret", "name")
	}

	summary.ServiceTemplates = ExtractServiceTemplates(obj)
	summary.CloudProvider = inferCloudProvider(obj, summary.TemplateRef.Name, summary.CredentialRef.Name)
	summary.Region = inferRegion(obj)

	summary.Ready = IsResourceReady(obj)
	if phase, _, err := unstructured.NestedString(obj.Object, "status", "phase"); err == nil {
		summary.Phase = phase
	}
	if message, _, err := unstructured.NestedString(obj.Object, "status", "message"); err == nil {
		summary.Message = message
	}
	summary.Conditions = extractConditions(obj)
	if summary.Message == "" {
		if msg := firstNonReadyConditionMessage(summary.Conditions); msg != "" {
			summary.Message = msg
		}
	}

	if summary.TemplateRef.Version == "" {
		if labels := obj.GetLabels(); labels != nil {
			if version := labels[labelTemplateVersion]; version != "" {
				summary.TemplateRef.Version = version
			}
		}
	}

	if url := inferManagementURL(obj); url != "" {
		summary.ManagementURL = url
	}

	return summary
}

// ExtractServiceTemplates returns referenced ServiceTemplate names in the deployment spec.
func ExtractServiceTemplates(obj *unstructured.Unstructured) []string {
	list, found, err := unstructured.NestedSlice(obj.Object, "spec", "serviceSpec", "services")
	if !found || err != nil {
		return nil
	}
	templates := make([]string, 0, len(list))
	for _, entry := range list {
		if m, ok := entry.(map[string]any); ok {
			if ref, ok := m["template"].(string); ok && ref != "" {
				templates = append(templates, ref)
			}
		}
	}
	return templates
}

// ServiceStatusSummary captures the deployment state of a single service on a cluster.
type ServiceStatusSummary struct {
	Name               string             `json:"name"`
	Namespace          string             `json:"namespace,omitempty"`
	Template           string             `json:"template"`
	State              string             `json:"state"` // Ready, Pending, Failed, Upgrading, etc.
	Type               string             `json:"type,omitempty"`
	Version            string             `json:"version,omitempty"`
	Conditions         []ConditionSummary `json:"conditions,omitempty"`
	LastTransitionTime *time.Time         `json:"lastTransitionTime,omitempty"`
}

// ExtractServiceStatuses extracts detailed per-service state information from .status.services.
func ExtractServiceStatuses(obj *unstructured.Unstructured) []ServiceStatusSummary {
	if obj == nil {
		return nil
	}

	list, found, err := unstructured.NestedSlice(obj.Object, "status", "services")
	if !found || err != nil || len(list) == 0 {
		return nil
	}

	services := make([]ServiceStatusSummary, 0, len(list))
	for _, entry := range list {
		svcMap, ok := entry.(map[string]any)
		if !ok {
			continue
		}

		svc := ServiceStatusSummary{
			Name:      asString(svcMap["name"]),
			Namespace: asString(svcMap["namespace"]),
			Template:  asString(svcMap["template"]),
			State:     asString(svcMap["state"]),
			Type:      asString(svcMap["type"]),
			Version:   asString(svcMap["version"]),
		}

		// Skip services with no name
		if svc.Name == "" {
			continue
		}

		// Extract lastTransitionTime
		if ts := asString(svcMap["lastTransitionTime"]); ts != "" {
			if parsed, err := time.Parse(time.RFC3339, ts); err == nil {
				svc.LastTransitionTime = &parsed
			}
		}

		// Extract conditions array
		if condList, ok := svcMap["conditions"].([]any); ok && len(condList) > 0 {
			svc.Conditions = make([]ConditionSummary, 0, len(condList))
			for _, condEntry := range condList {
				condMap, ok := condEntry.(map[string]any)
				if !ok {
					continue
				}
				cond := ConditionSummary{
					Type:    asString(condMap["type"]),
					Status:  asString(condMap["status"]),
					Reason:  asString(condMap["reason"]),
					Message: asString(condMap["message"]),
				}
				if condTs := asString(condMap["lastTransitionTime"]); condTs != "" {
					if parsed, err := time.Parse(time.RFC3339, condTs); err == nil {
						cond.LastTransitionTime = &parsed
					}
				}
				svc.Conditions = append(svc.Conditions, cond)
			}
		}

		services = append(services, svc)
	}

	return services
}

func buildTemplateReference(obj *unstructured.Unstructured, defaultNS string) ResourceReference {
	ref := buildReferenceFromPath(obj, defaultNS, "spec", "template")
	if ref.Version == "" {
		ref.Version = inferTemplateVersion(obj, ref.Name)
	}
	return ref
}

func buildReferenceFromPath(obj *unstructured.Unstructured, defaultNS string, path ...string) ResourceReference {
	value, found, err := unstructured.NestedString(obj.Object, path...)
	if err != nil || !found || value == "" {
		return ResourceReference{}
	}
	ref := ResourceReference{Name: value}
	if strings.Contains(value, "/") {
		parts := strings.SplitN(value, "/", 2)
		ref.Namespace = parts[0]
		ref.Name = parts[1]
	} else if defaultNS != "" {
		ref.Namespace = defaultNS
	}
	return ref
}

func buildClusterIdentityReference(obj *unstructured.Unstructured, defaultNS string) ResourceReference {
	identity, found, err := unstructured.NestedMap(obj.Object, "spec", "config", "clusterIdentity")
	if err != nil || !found {
		return ResourceReference{}
	}
	name, _ := identity["name"].(string)
	if name == "" {
		return ResourceReference{}
	}
	ns, _ := identity["namespace"].(string)
	if ns == "" {
		ns = defaultNS
	}
	return ResourceReference{
		Name:      name,
		Namespace: ns,
	}
}

func inferCloudProvider(obj *unstructured.Unstructured, templateName, credentialName string) string {
	labels := obj.GetLabels()
	if provider := lookupLabel(labels, labelCloudProvider, labelLegacyProvider); provider != "" {
		return provider
	}
	if provider := firstSegment(templateName); provider != "" {
		return provider
	}
	if provider := firstSegment(credentialName); provider != "" {
		return provider
	}
	return ""
}

func inferRegion(obj *unstructured.Unstructured) string {
	if labels := obj.GetLabels(); labels != nil {
		if region := labels[labelCloudRegion]; region != "" {
			return region
		}
	}
	if region, found, err := unstructured.NestedString(obj.Object, "spec", "config", "location"); err == nil && found {
		return region
	}
	return ""
}

func inferManagementURL(obj *unstructured.Unstructured) string {
	annotations := obj.GetAnnotations()
	if annotations == nil {
		return ""
	}
	if url := annotations[annotationMgmtURL]; url != "" {
		return url
	}
	return annotations[annotationCloudURL]
}

func extractOwner(obj *unstructured.Unstructured) string {
	if annotations := obj.GetAnnotations(); annotations != nil {
		if owner := annotations[annotationOwner]; owner != "" {
			return owner
		}
	}
	if owners := obj.GetOwnerReferences(); len(owners) > 0 {
		return owners[0].Name
	}
	return ""
}

func extractConditions(obj *unstructured.Unstructured) []ConditionSummary {
	list, found, err := unstructured.NestedSlice(obj.Object, "status", "conditions")
	if err != nil || !found || len(list) == 0 {
		return nil
	}
	conditions := make([]ConditionSummary, 0, len(list))
	for _, entry := range list {
		condMap, ok := entry.(map[string]any)
		if !ok {
			continue
		}
		cond := ConditionSummary{
			Type:    asString(condMap["type"]),
			Status:  asString(condMap["status"]),
			Reason:  asString(condMap["reason"]),
			Message: asString(condMap["message"]),
		}
		if ts := asString(condMap["lastTransitionTime"]); ts != "" {
			if parsed, err := time.Parse(time.RFC3339, ts); err == nil {
				cond.LastTransitionTime = &parsed
			}
		}
		conditions = append(conditions, cond)
	}
	return conditions
}

func firstNonReadyConditionMessage(conditions []ConditionSummary) string {
	for _, cond := range conditions {
		if strings.EqualFold(cond.Status, "true") {
			continue
		}
		if cond.Message != "" {
			return cond.Message
		}
	}
	return ""
}

func inferTemplateVersion(obj *unstructured.Unstructured, templateName string) string {
	if labels := obj.GetLabels(); labels != nil {
		if version := labels[labelTemplateVersion]; version != "" {
			return version
		}
	}
	if idx := strings.LastIndex(templateName, "-"); idx != -1 {
		candidate := templateName[idx+1:]
		if looksLikeVersion(candidate) {
			return candidate
		}
	}
	return ""
}

func looksLikeVersion(value string) bool {
	if value == "" {
		return false
	}
	hasDigit := false
	for i := 0; i < len(value); i++ {
		c := value[i]
		switch {
		case c >= '0' && c <= '9':
			hasDigit = true
		case c == '.' || c == '-' || c == '_':
		default:
			return false
		}
	}
	return hasDigit
}

func firstSegment(value string) string {
	if value == "" {
		return ""
	}
	value = strings.TrimSpace(value)
	if idx := strings.Index(value, "/"); idx != -1 {
		value = value[:idx]
	}
	if idx := strings.Index(value, "-"); idx != -1 {
		value = value[:idx]
	}
	return strings.ToLower(value)
}

func lookupLabel(labels map[string]string, preferred string, fallback string) string {
	if labels == nil {
		return ""
	}
	if val := labels[preferred]; val != "" {
		return val
	}
	return labels[fallback]
}

func asString(value any) string {
	if value == nil {
		return ""
	}
	if str, ok := value.(string); ok {
		return str
	}
	return ""
}
