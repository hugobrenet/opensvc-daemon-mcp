package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	mcptools "github.com/hugobrenet/opensvc-daemon-mcp/internal/tools"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestServerOverStdio(t *testing.T) {
	const token = "test-daemon-jwt"
	tokenFile := filepath.Join(t.TempDir(), "daemon.jwt")
	if err := os.WriteFile(tokenFile, []byte(token+"\n"), 0o600); err != nil {
		t.Fatalf("write daemon JWT file: %v", err)
	}

	daemonServer := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/api/cluster/status" {
			t.Errorf("got daemon path %q, want /api/cluster/status", request.URL.Path)
		}
		if got := request.Header.Get("Authorization"); got != "Bearer "+token {
			http.Error(response, "unauthorized", http.StatusUnauthorized)
			return
		}
		response.Header().Set("Content-Type", "application/json")
		fmt.Fprint(response, `{
			"cluster": {
				"config": {
					"id": "cluster-123",
					"name": "prod"
				},
				"node": {
					"node-a": {
						"status": {
							"agent": "v3.0.0"
						}
					}
				}
			},
			"daemon": {
				"nodename": "node-a"
			}
		}`)
	}))
	defer daemonServer.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	command := exec.CommandContext(ctx, "go", "run", ".")
	command.Env = append(
		os.Environ(),
		"OPENSVC_DAEMON_URL="+daemonServer.URL,
		"OPENSVC_DAEMON_AUTH_METHOD=jwt",
		"OPENSVC_DAEMON_TOKEN_FILE="+tokenFile,
	)

	client := mcp.NewClient(
		&mcp.Implementation{
			Name:    "opensvc-daemon-mcp-test",
			Version: "v0.1.0",
		},
		nil,
	)
	session, err := client.Connect(
		ctx,
		&mcp.CommandTransport{Command: command},
		nil,
	)
	if err != nil {
		t.Fatalf("connect to MCP server: %v", err)
	}
	defer func() {
		if err := session.Close(); err != nil {
			t.Errorf("close MCP session: %v", err)
		}
	}()

	tools, err := session.ListTools(ctx, &mcp.ListToolsParams{})
	if err != nil {
		t.Fatalf("list MCP tools: %v", err)
	}
	if len(tools.Tools) != 1 {
		t.Fatalf("got %d tools, want 1", len(tools.Tools))
	}
	if got := tools.Tools[0].Name; got != "get_server_identity" {
		t.Fatalf("got tool %q, want %q", got, "get_server_identity")
	}

	result, err := session.CallTool(ctx, &mcp.CallToolParams{
		Name:      "get_server_identity",
		Arguments: mcptools.GetServerIdentityInput{},
	})
	if err != nil {
		t.Fatalf("call get_server_identity: %v", err)
	}
	if result.IsError {
		t.Fatalf("get_server_identity returned an MCP tool error: %#v", result.Content)
	}

	data, err := json.Marshal(result.StructuredContent)
	if err != nil {
		t.Fatalf("marshal structured content: %v", err)
	}
	var identity mcptools.GetServerIdentityOutput
	if err := json.Unmarshal(data, &identity); err != nil {
		t.Fatalf("decode structured content: %v", err)
	}

	t.Logf(
		"get_server_identity response: nodename=%q cluster_id=%q cluster_name=%q agent_version=%q",
		identity.Daemon.NodeName,
		identity.Cluster.ID,
		identity.Cluster.Name,
		identity.Node.AgentVersion,
	)
	if identity.Daemon.NodeName != "node-a" {
		t.Errorf("got nodename %q, want node-a", identity.Daemon.NodeName)
	}
	if identity.Cluster.ID != "cluster-123" {
		t.Errorf("got cluster ID %q, want cluster-123", identity.Cluster.ID)
	}
	if identity.Cluster.Name != "prod" {
		t.Errorf("got cluster name %q, want prod", identity.Cluster.Name)
	}
	if identity.Node.AgentVersion != "v3.0.0" {
		t.Errorf("got agent version %q, want v3.0.0", identity.Node.AgentVersion)
	}
}
