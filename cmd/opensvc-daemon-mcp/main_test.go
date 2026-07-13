package main

import (
	"context"
	"encoding/json"
	"os/exec"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestServerOverStdio(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	client := mcp.NewClient(
		&mcp.Implementation{
			Name:    "opensvc-daemon-mcp-test",
			Version: "v0.1.0",
		},
		nil,
	)
	session, err := client.Connect(
		ctx,
		&mcp.CommandTransport{Command: exec.CommandContext(ctx, "go", "run", ".")},
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
		Arguments: GetServerIdentityInput{},
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
	var identity GetServerIdentityOutput
	if err := json.Unmarshal(data, &identity); err != nil {
		t.Fatalf("decode structured content: %v", err)
	}
	t.Logf("get_server_identity response: name=%q version=%q", identity.Name, identity.Version)
	if identity.Name != serverName {
		t.Errorf("got server name %q, want %q", identity.Name, serverName)
	}
	if identity.Version != serverVersion {
		t.Errorf("got server version %q, want %q", identity.Version, serverVersion)
	}
}
