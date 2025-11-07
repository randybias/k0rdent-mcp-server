//go:build live

package integration

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
)

const protocolVersion = "2025-06-18"

type liveClient struct {
	endpoint   string
	httpClient *http.Client
	sessionID  string
}

func newLiveClient(t testing.TB) *liveClient {
	t.Helper()
	_, endpoint := requireLiveEnv(t)
	client := &liveClient{
		endpoint:   endpoint,
		httpClient: &http.Client{Timeout: 30 * time.Second},
	}
	client.sessionID = client.initialize(t)
	return client
}

func (c *liveClient) initialize(t testing.TB) string {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	reqBody := map[string]any{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "initialize",
		"params": map[string]any{
			"protocolVersion": protocolVersion,
			"clientInfo":      map[string]any{"name": "live-test", "version": "dev"},
			"capabilities":    map[string]any{},
		},
	}
	payload, err := json.Marshal(reqBody)
	if err != nil {
		t.Fatalf("marshal initialize request: %v", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.endpoint, bytes.NewReader(payload))
	if err != nil {
		t.Fatalf("create initialize request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json, text/event-stream")
	req.Header.Set("Mcp-Protocol-Version", protocolVersion)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		t.Fatalf("%s", formatLiveFailure("initialize", err))
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		buf := new(bytes.Buffer)
		_, _ = buf.ReadFrom(resp.Body)
		t.Fatalf("initialize returned status %d: %s", resp.StatusCode, strings.TrimSpace(buf.String()))
	}

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read initialize response: %v", err)
	}

	payloadBytes, err := extractSSEPayload(bodyBytes)
	if err != nil {
		t.Fatalf("parse initialize SSE: %v (body: %s)", err, string(bodyBytes))
	}

	var rpcResp struct {
		Result struct {
			ProtocolVersion string `json:"protocolVersion"`
		} `json:"result"`
	}
	if err := json.Unmarshal(payloadBytes, &rpcResp); err != nil {
		t.Fatalf("decode initialize response: %v (payload: %s)", err, string(payloadBytes))
	}

	sessionID := resp.Header.Get("Mcp-Session-Id")
	if sessionID == "" {
		t.Fatalf("initialize response missing Mcp-Session-Id header")
	}

	return sessionID
}

func (c *liveClient) CallTool(t testing.TB, name string, arguments map[string]any) json.RawMessage {
	t.Helper()
	result, err := c.callToolInternal(name, arguments)
	if err != nil {
		t.Fatalf("%v", err)
	}
	return result
}

// CallToolSafe calls an MCP tool but returns errors instead of failing the test.
// Use this for cleanup operations where failures should be logged but not fail the test.
func (c *liveClient) CallToolSafe(name string, arguments map[string]any) (json.RawMessage, error) {
	return c.callToolInternal(name, arguments)
}

// callToolInternal is the shared implementation for CallTool and CallToolSafe
func (c *liveClient) callToolInternal(name string, arguments map[string]any) (json.RawMessage, error) {
	reqBody := map[string]any{
		"jsonrpc": "2.0",
		"id":      time.Now().UnixNano(),
		"method":  "tools/call",
		"params": map[string]any{
			"name":      name,
			"arguments": arguments,
		},
	}

	payload, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("marshal call request: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.endpoint, bytes.NewReader(payload))
	if err != nil {
		return nil, fmt.Errorf("create call request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json, text/event-stream")
	req.Header.Set("Mcp-Protocol-Version", protocolVersion)
	req.Header.Set("Mcp-Session-Id", c.sessionID)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("%s", formatLiveFailure(fmt.Sprintf("call tool %s", name), err))
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		buf := new(bytes.Buffer)
		_, _ = buf.ReadFrom(resp.Body)
		return nil, fmt.Errorf("%s returned status %d: %s", name, resp.StatusCode, strings.TrimSpace(buf.String()))
	}

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read %s response: %w", name, err)
	}

	payloadBytes, err := extractSSEPayload(bodyBytes)
	if err != nil {
		return nil, fmt.Errorf("parse SSE: %w (body: %s)", err, string(bodyBytes))
	}

	var rpcResp struct {
		Result struct {
			Structured json.RawMessage `json:"structuredContent"`
			Output     []struct {
				Type string          `json:"type"`
				Text string          `json:"text"`
				JSON json.RawMessage `json:"json"`
			} `json:"output"`
		} `json:"result"`
	}
	if err := json.Unmarshal(payloadBytes, &rpcResp); err != nil {
		return nil, fmt.Errorf("decode %s response: %w (payload: %s)", name, err, string(payloadBytes))
	}

	if rpcResp.Result.Structured == nil {
		for _, out := range rpcResp.Result.Output {
			if len(out.JSON) > 0 {
				return out.JSON, nil
			}
			if out.Text != "" {
				return json.RawMessage(fmt.Sprintf(`"%s"`, out.Text)), nil
			}
		}
		return nil, fmt.Errorf("%s returned no structured/json output (payload=%s)", name, string(payloadBytes))
	}

	return rpcResp.Result.Structured, nil
}

func extractSSEPayload(body []byte) ([]byte, error) {
	lines := strings.Split(string(body), "\n")
	var dataLines []string
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "data:") {
			dataLines = append(dataLines, strings.TrimSpace(strings.TrimPrefix(line, "data:")))
		}
	}
	if len(dataLines) == 0 {
		return nil, fmt.Errorf("no data lines found in SSE payload")
	}
	combined := strings.Join(dataLines, "\n")
	return []byte(combined), nil
}
