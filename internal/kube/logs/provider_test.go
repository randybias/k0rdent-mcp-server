package logs

import (
	"context"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

func TestResolveContainerSingle(t *testing.T) {
	client := fake.NewSimpleClientset(&corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Namespace: "ns", Name: "pod"},
		Spec:       corev1.PodSpec{Containers: []corev1.Container{{Name: "app"}}},
	})

	provider, err := NewProvider(client)
	if err != nil {
		t.Fatalf("NewProvider error: %v", err)
	}

	container, err := provider.resolveContainer(context.Background(), "ns", "pod", "")
	if err != nil {
		t.Fatalf("resolveContainer error: %v", err)
	}
	if container != "app" {
		t.Fatalf("expected app, got %s", container)
	}
}

func TestResolveContainerRequiresExplicit(t *testing.T) {
	client := fake.NewSimpleClientset(&corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Namespace: "ns", Name: "pod"},
		Spec: corev1.PodSpec{
			Containers:     []corev1.Container{{Name: "app"}},
			InitContainers: []corev1.Container{{Name: "init"}},
		},
	})

	provider, err := NewProvider(client)
	if err != nil {
		t.Fatalf("NewProvider error: %v", err)
	}

	if _, err := provider.resolveContainer(context.Background(), "ns", "pod", ""); err == nil {
		t.Fatalf("expected error when multiple containers present")
	}
}
