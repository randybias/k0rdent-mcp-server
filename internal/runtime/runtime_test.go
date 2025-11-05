package runtime

import (
	"bytes"
	"context"
	"io"
	"log/slog"
	"regexp"
	"sync"
	"testing"
	"time"

	"github.com/k0rdent/mcp-k0rdent-server/internal/config"
	"github.com/k0rdent/mcp-k0rdent-server/internal/kube"
	eventsprovider "github.com/k0rdent/mcp-k0rdent-server/internal/kube/events"
	logsprovider "github.com/k0rdent/mcp-k0rdent-server/internal/kube/logs"
	"github.com/k0rdent/mcp-k0rdent-server/internal/logging"

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

	factory, err := kube.NewClientFactory(&rest.Config{Host: "https://example.com"}, slog.New(slog.NewJSONHandler(io.Discard, nil)))
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

	rt, err := New(settings, factory, slog.New(slog.NewJSONHandler(io.Discard, nil)))
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

func TestRuntimeSessionLogs(t *testing.T) {
	var buf bytes.Buffer
	sink := &recordingSink{}
	mgr := logging.NewManager(logging.Options{
		Level:       slog.LevelDebug,
		Sink:        sink,
		Destination: &buf,
	})
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		_ = mgr.Close(ctx)
	})

	settings := &config.Settings{}
	factory, err := kube.NewClientFactory(&rest.Config{Host: "https://example.com"}, mgr.Logger())
	if err != nil {
		t.Fatalf("NewClientFactory error: %v", err)
	}

	factory.WithConstructors(
		func(*rest.Config) (kubernetes.Interface, error) {
			return fake.NewSimpleClientset(), nil
		},
		func(*rest.Config) (dynamic.Interface, error) {
			return dynamicfake.NewSimpleDynamicClient(apiruntime.NewScheme()), nil
		},
	)

	rt, err := New(settings, factory, mgr.Logger())
	if err != nil {
		t.Fatalf("New error: %v", err)
	}

	rt.newEventProvider = func(context.Context, kubernetes.Interface) (*eventsprovider.Provider, error) {
		return &eventsprovider.Provider{}, nil
	}
	rt.newLogProvider = func(kubernetes.Interface) (*logsprovider.Provider, error) {
		return &logsprovider.Provider{}, nil
	}

	if _, err := rt.NewSession(context.Background(), "token"); err != nil {
		t.Fatalf("NewSession error: %v", err)
	}

	waitUntil(func() bool { return sink.Len() >= 2 }, 500*time.Millisecond)
	if sink.Len() == 0 {
		t.Fatalf("expected sink to capture runtime logs")
	}
	if !bytes.Contains(buf.Bytes(), []byte(`"component":"runtime"`)) {
		t.Fatalf("expected stdout log to include component attribute, got %s", buf.String())
	}
}

type recordingSink struct {
	mu      sync.Mutex
	entries []logging.Entry
}

func (s *recordingSink) Write(_ context.Context, e logging.Entry) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.entries = append(s.entries, e)
	return nil
}

func (s *recordingSink) Len() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.entries)
}

func waitUntil(fn func() bool, timeout time.Duration) {
	deadline := time.Now().Add(timeout)
	for !fn() && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
}
