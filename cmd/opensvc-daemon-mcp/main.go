package main

import (
	"context"
	"log"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

const (
	serverName    = "opensvc-daemon-mcp"
	serverVersion = "v0.1.0"
)

type GetServerIdentityInput struct{}

type GetServerIdentityOutput struct {
	Name    string `json:"name" jsonschema:"the MCP server name"`
	Version string `json:"version" jsonschema:"the MCP server version"`
}

func getServerIdentity(
	ctx context.Context,
	req *mcp.CallToolRequest,
	input GetServerIdentityInput,
) (*mcp.CallToolResult, GetServerIdentityOutput, error) {
	return nil, GetServerIdentityOutput{
		Name:    serverName,
		Version: serverVersion,
	}, nil
}

func main() {
	server := mcp.NewServer(
		&mcp.Implementation{
			Name:    serverName,
			Version: serverVersion,
		},
		nil,
	)
	mcp.AddTool(server, &mcp.Tool{
		Name:        "get_server_identity",
		Description: "Return the identity and version of the OpenSVC daemon MCP server.",
	}, getServerIdentity)

	if err := server.Run(context.Background(), &mcp.StdioTransport{}); err != nil {
		log.Fatal(err)
	}
}
