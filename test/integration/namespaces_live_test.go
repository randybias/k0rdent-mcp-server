//go:build live

package integration

import (
    "encoding/json"
    "testing"
)

type namespaceListResult struct {
    Namespaces []struct {
        Name string `json:"name"`
    } `json:"namespaces"`
}

func TestNamespacesListLive(t *testing.T) {
    client := newLiveClient(t)

    raw := client.CallTool(t, "k0rdent.mgmt.namespaces.list", map[string]any{})

    var result namespaceListResult
    if err := json.Unmarshal(raw, &result); err != nil {
        t.Fatalf("decode namespaces result: %v", err)
    }
    if len(result.Namespaces) == 0 {
        t.Fatalf("expected at least one namespace in cluster")
    }
}
