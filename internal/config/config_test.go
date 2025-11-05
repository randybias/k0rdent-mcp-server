package config

import (
	"context"
	"errors"
	"strings"
	"testing"

	"k8s.io/client-go/rest"
)

func TestLoadFromPath(t *testing.T) {
	env := map[string]string{
		envKubeconfigPath: "/tmp/kubeconfig",
		envContext:        "dev",
		envNamespaceExpr:  "^team-",
		envAuthMode:       string(AuthModeOIDCRequired),
	}

	kubeconfigYAML := strings.TrimSpace(`
apiVersion: v1
clusters:
- cluster:
    server: https://example.com
  name: prod
contexts:
- context:
    cluster: prod
    user: default
  name: prod
- context:
    cluster: prod
    user: dev-user
  name: dev
current-context: prod
users:
- name: default
  user:
    token: prod-token
- name: dev-user
  user:
    token: dev-token
`)

	loader := &Loader{
		envLookup: func(key string) (string, bool) {
			val, ok := env[key]
			return val, ok
		},
		readFile: func(path string) ([]byte, error) {
			if path != "/tmp/kubeconfig" {
				t.Fatalf("unexpected path %q", path)
			}
			return []byte(kubeconfigYAML), nil
		},
		ping: func(ctx context.Context, cfg *rest.Config) error {
			if cfg == nil {
				t.Fatal("rest config is nil")
			}
			if got, want := cfg.Host, "https://example.com"; got != want {
				t.Fatalf("unexpected host: got %q want %q", got, want)
			}
			return nil
		},
	}

	settings, err := loader.Load(context.Background())
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}

	if settings.Source != SourcePath {
		t.Fatalf("expected SourcePath, got %q", settings.Source)
	}
	if settings.ContextName != "dev" {
		t.Fatalf("expected context %q, got %q", "dev", settings.ContextName)
	}
	if settings.AuthMode != AuthModeOIDCRequired {
		t.Fatalf("expected auth mode %q, got %q", AuthModeOIDCRequired, settings.AuthMode)
	}
	if settings.NamespaceFilter == nil {
		t.Fatal("expected namespace filter regex to be compiled")
	}
	if !settings.NamespaceFilter.MatchString("team-alpha") {
		t.Fatal("compiled namespace filter does not match expected value")
	}
	if settings.NamespaceFilter.MatchString("other") {
		t.Fatal("compiled namespace filter should not match 'other'")
	}
}

func TestLoadRejectsInvalidBase64(t *testing.T) {
	env := map[string]string{
		envKubeconfigB64: "!!!notbase64!!!",
	}

	loader := &Loader{
		envLookup: func(key string) (string, bool) {
			val, ok := env[key]
			return val, ok
		},
		readFile: func(string) ([]byte, error) {
			return nil, errors.New("should not be called")
		},
		ping: func(context.Context, *rest.Config) error {
			return errors.New("should not be called")
		},
	}

	_, err := loader.Load(context.Background())
	if err == nil {
		t.Fatal("expected error for invalid base64 input")
	}
	if !strings.Contains(err.Error(), "decode K0RDENT_MGMT_KUBECONFIG_B64") {
		t.Fatalf("expected decode error, got %v", err)
	}
}

func TestLoadRejectsMultipleSources(t *testing.T) {
	env := map[string]string{
		envKubeconfigPath: "/tmp/config",
		envKubeconfigText: "fake",
	}

	loader := &Loader{
		envLookup: func(key string) (string, bool) {
			val, ok := env[key]
			return val, ok
		},
		readFile: func(string) ([]byte, error) {
			return nil, errors.New("should not be called")
		},
		ping: func(context.Context, *rest.Config) error {
			return errors.New("should not be called")
		},
	}

	_, err := loader.Load(context.Background())
	if err == nil {
		t.Fatal("expected error when multiple kubeconfig sources are set")
	}
	if !strings.Contains(err.Error(), "only one of") {
		t.Fatalf("expected only one source error, got %v", err)
	}
}

func TestLoadRejectsInvalidNamespaceRegex(t *testing.T) {
	env := map[string]string{
		envKubeconfigText: minimalKubeconfig(),
		envNamespaceExpr:  "(",
	}

	loader := &Loader{
		envLookup: func(key string) (string, bool) {
			val, ok := env[key]
			return val, ok
		},
		readFile: func(string) ([]byte, error) {
			return nil, errors.New("should not be called")
		},
		ping: func(context.Context, *rest.Config) error {
			return nil
		},
	}

	_, err := loader.Load(context.Background())
	if err == nil {
		t.Fatal("expected error for invalid namespace regex")
	}
	if !strings.Contains(err.Error(), "compile namespace filter regex") {
		t.Fatalf("expected namespace regex error, got %v", err)
	}
}

func TestLoadRejectsInvalidAuthMode(t *testing.T) {
	env := map[string]string{
		envKubeconfigText: minimalKubeconfig(),
		envAuthMode:       "NOT_VALID",
	}

	loader := &Loader{
		envLookup: func(key string) (string, bool) {
			val, ok := env[key]
			return val, ok
		},
		readFile: func(string) ([]byte, error) {
			return nil, errors.New("should not be called")
		},
		ping: func(context.Context, *rest.Config) error {
			return nil
		},
	}

	_, err := loader.Load(context.Background())
	if err == nil {
		t.Fatal("expected error for invalid AUTH_MODE")
	}
	if !strings.Contains(err.Error(), "invalid AUTH_MODE") {
		t.Fatalf("expected invalid AUTH_MODE error, got %v", err)
	}
}

func minimalKubeconfig() string {
	return strings.TrimSpace(`
apiVersion: v1
clusters:
- cluster:
    server: https://example.com
  name: main
contexts:
- context:
    cluster: main
    user: default
  name: main
current-context: main
users:
- name: default
  user:
    token: token
`)
}
