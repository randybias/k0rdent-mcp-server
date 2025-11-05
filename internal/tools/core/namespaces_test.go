package core

import (
	"bytes"
	"context"
	"log/slog"
	"regexp"
	"sync"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"

	"github.com/k0rdent/mcp-k0rdent-server/internal/logging"
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

func TestNamespacesHandleLogs(t *testing.T) {
	clientset := fake.NewSimpleClientset(&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "default"}})
	var buf bytes.Buffer
	sink := &recordingSink{}
	mgr := logging.NewManager(logging.Options{
		Level:       slog.LevelDebug,
		Sink:        sink,
		Destination: &buf,
	})
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		_ = mgr.Close(ctx)
	}()

	session := &runtime.Session{
		Clients: runtime.Clients{Kubernetes: clientset},
		Logger:  logging.WithComponent(mgr.Logger(), "runtime"),
	}

	tool := &namespacesTool{session: session}
	req := &mcp.CallToolRequest{Params: &mcp.CallToolParamsRaw{Name: "k0.namespaces.list"}}
	if _, _, err := tool.handle(context.Background(), req, namespaceListInput{}); err != nil {
		t.Fatalf("handle returned error: %v", err)
	}
	deadline := time.Now().Add(500 * time.Millisecond)
	for sink.Len() == 0 && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	if sink.Len() == 0 {
		t.Fatalf("expected log entries to be captured; stdout: %s", buf.String())
	}
	entry := sink.Last()
	if entry.Message != "namespaces listed" {
		t.Fatalf("unexpected log message %q", entry.Message)
	}
	if entry.Attributes["tool"] != "k0.namespaces.list" {
		t.Fatalf("expected tool attribute, got %#v", entry.Attributes)
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

func (s *recordingSink) Last() logging.Entry {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.entries[len(s.entries)-1]
}
