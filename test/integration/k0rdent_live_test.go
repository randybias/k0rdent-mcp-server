//go:build live

package integration

import (
    "encoding/json"
    "testing"
)

type k0ListResult struct {
    Items []map[string]any `json:"items"`
}

func TestK0rdentCRDsLive(t *testing.T) {
    client := newLiveClient(t)

    tools := []string{
        "k0rdent.mgmt.serviceTemplates.list",
        "k0rdent.mgmt.clusterDeployments.listAll",
        "k0rdent.mgmt.multiClusterServices.list",
    }

    for _, tool := range tools {
        raw := client.CallTool(t, tool, map[string]any{})
        var result k0ListResult
        if err := json.Unmarshal(raw, &result); err != nil {
            t.Fatalf("decode result for %s: %v", tool, err)
        }
        if result.Items == nil {
            t.Fatalf("%s returned nil items", tool)
        }
    }
}
