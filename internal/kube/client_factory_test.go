package kube

import (
	"errors"
	"testing"

	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

func TestRESTConfigForToken(t *testing.T) {
	base := &rest.Config{
		Host:        "https://example.com",
		BearerToken: "base-token",
		Username:    "user",
		Password:    "pass",
	}

	factory, err := NewClientFactory(base)
	if err != nil {
		t.Fatalf("NewClientFactory returned error: %v", err)
	}

	cfg, err := factory.RESTConfigForToken("override")
	if err != nil {
		t.Fatalf("RESTConfigForToken returned error: %v", err)
	}

	if cfg == base {
		t.Fatal("expected REST config to be copied, but references match")
	}
	if got, want := cfg.BearerToken, "override"; got != want {
		t.Fatalf("unexpected bearer token: got %q want %q", got, want)
	}
	if cfg.BearerTokenFile != "" {
		t.Fatalf("expected bearer token file to be cleared, got %q", cfg.BearerTokenFile)
	}
	if cfg.Username != "" || cfg.Password != "" {
		t.Fatalf("expected basic auth credentials to be cleared, got user=%q pass=%q", cfg.Username, cfg.Password)
	}

	if base.BearerToken != "base-token" {
		t.Fatalf("expected base config bearer token to remain unchanged, got %q", base.BearerToken)
	}
	if base.Username != "user" || base.Password != "pass" {
		t.Fatalf("expected base basic auth credentials to remain unchanged, got user=%q pass=%q", base.Username, base.Password)
	}
}

func TestKubernetesClientDelegatesToConstructor(t *testing.T) {
	base := &rest.Config{
		Host: "https://example.com",
	}

	factory, err := NewClientFactory(base)
	if err != nil {
		t.Fatalf("NewClientFactory returned error: %v", err)
	}

	var capturedToken string
	stub := func(cfg *rest.Config) (kubernetes.Interface, error) {
		capturedToken = cfg.BearerToken
		return nil, nil
	}

	factory.WithConstructors(stub, nil)

	if _, err := factory.KubernetesClient("from-request"); err != nil {
		t.Fatalf("KubernetesClient returned error: %v", err)
	}

	if capturedToken != "from-request" {
		t.Fatalf("expected constructor to receive overridden bearer token, got %q", capturedToken)
	}
}

func TestDynamicClientDelegatesToConstructor(t *testing.T) {
	base := &rest.Config{
		Host: "https://example.com",
	}

	factory, err := NewClientFactory(base)
	if err != nil {
		t.Fatalf("NewClientFactory returned error: %v", err)
	}

	var capturedHost string
	stub := func(cfg *rest.Config) (dynamic.Interface, error) {
		capturedHost = cfg.Host
		return nil, errors.New("stop here")
	}

	factory.WithConstructors(nil, stub)

	if _, err := factory.DynamicClient(""); err == nil || err.Error() != "create dynamic client: stop here" {
		t.Fatalf("expected wrapped constructor error, got %v", err)
	}

	if capturedHost != "https://example.com" {
		t.Fatalf("expected constructor to receive copied config, got host %q", capturedHost)
	}
}
