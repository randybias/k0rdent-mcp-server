package core

import (
	"context"
	"regexp"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"

	"github.com/k0rdent/mcp-k0rdent-server/internal/runtime"
)

func TestNamespacesHandle(t *testing.T) {
	clientset := fake.NewSimpleClientset(
		&corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name:   "team-alpha",
				Labels: map[string]string{"env": "dev"},
			},
			Status: corev1.NamespaceStatus{Phase: corev1.NamespaceActive},
		},
		&corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: "platform",
			},
			Status: corev1.NamespaceStatus{Phase: corev1.NamespaceTerminating},
		},
	)

	tool := &namespacesTool{
		session: &runtime.Session{
			NamespaceFilter: regexp.MustCompile("^team-"),
			Clients: runtime.Clients{
				Kubernetes: clientset,
			},
		},
	}

	_, result, err := tool.handle(context.Background(), nil, namespaceListInput{})
	if err != nil {
		t.Fatalf("handle returned error: %v", err)
	}
	if len(result.Namespaces) != 1 {
		t.Fatalf("expected 1 namespace, got %d", len(result.Namespaces))
	}
	ns := result.Namespaces[0]
	if ns.Name != "team-alpha" {
		t.Fatalf("unexpected namespace name %q", ns.Name)
	}
	if ns.Status != string(corev1.NamespaceActive) {
		t.Fatalf("unexpected status %q", ns.Status)
	}
	if ns.Labels["env"] != "dev" {
		t.Fatalf("expected label env=dev, got %#v", ns.Labels)
	}
}
