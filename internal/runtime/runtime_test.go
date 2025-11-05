package runtime

import (
	"context"
	"regexp"
	"testing"

	"github.com/k0rdent/mcp-k0rdent-server/internal/config"
	"github.com/k0rdent/mcp-k0rdent-server/internal/kube"
	eventsprovider "github.com/k0rdent/mcp-k0rdent-server/internal/kube/events"
	logsprovider "github.com/k0rdent/mcp-k0rdent-server/internal/kube/logs"

	apiruntime "k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/dynamic"
	dynamicfake "k8s.io/client-go/dynamic/fake"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/rest"
)

func TestNewSession(t *testing.T) {
	settings := &config.Settings{
		NamespaceFilter: regexp.MustCompile("^team-"),
	}

	factory, err := kube.NewClientFactory(&rest.Config{Host: "https://example.com"})
	if err != nil {
		t.Fatalf("NewClientFactory returned error: %v", err)
	}

	factory.WithConstructors(
		func(*rest.Config) (kubernetes.Interface, error) {
			return fake.NewSimpleClientset(), nil
		},
		func(*rest.Config) (dynamic.Interface, error) {
			return dynamicfake.NewSimpleDynamicClient(apiruntime.NewScheme()), nil
		},
	)

	rt, err := New(settings, factory, nil)
	if err != nil {
		t.Fatalf("New returned error: %v", err)
	}

	rt.newEventProvider = func(context.Context, kubernetes.Interface) (*eventsprovider.Provider, error) {
		return &eventsprovider.Provider{}, nil
	}
	rt.newLogProvider = func(kubernetes.Interface) (*logsprovider.Provider, error) {
		return &logsprovider.Provider{}, nil
	}

	session, err := rt.NewSession(context.Background(), "token")
	if err != nil {
		t.Fatalf("NewSession returned error: %v", err)
	}
	if session.NamespaceFilter == nil || !session.NamespaceFilter.MatchString("team-alpha") {
		t.Fatalf("namespace filter not propagated")
	}
	if session.Clients.Kubernetes == nil {
		t.Fatalf("expected kubernetes client")
	}
	if session.Clients.Dynamic == nil {
		t.Fatalf("expected dynamic client")
	}
	if session.Events == nil {
		t.Fatalf("expected events provider")
	}
	if session.Logs == nil {
		t.Fatalf("expected logs provider")
	}
}
