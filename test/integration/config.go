//go:build live

package integration

import (
	"fmt"
	"os"
	"testing"
)

const (
	envKubeconfig = "K0RDENT_MGMT_KUBECONFIG_PATH"
	envAuthMode   = "AUTH_MODE"
	envEndpoint   = "K0RDENT_MCP_ENDPOINT"
)

func requireLiveEnv(t testing.TB) (kubeconfig, endpoint string) {
	t.Helper()

	kubeconfig = os.Getenv(envKubeconfig)
	if kubeconfig == "" {
		t.Fatalf("%s must be set for live tests", envKubeconfig)
	}
	if _, err := os.Stat(kubeconfig); err != nil {
		t.Fatalf("unable to access kubeconfig %s: %v", kubeconfig, err)
	}

	if os.Getenv(envAuthMode) == "" {
		t.Fatalf("%s must be set for live tests", envAuthMode)
	}

	endpoint = os.Getenv(envEndpoint)
	if endpoint == "" {
		endpoint = "http://127.0.0.1:6767/mcp"
	}
	return kubeconfig, endpoint
}

func formatLiveFailure(entity string, err error) string {
	return fmt.Sprintf("live %s failed: %v", entity, err)
}
