package tools

import (
	"context"

	"github.com/hugobrenet/opensvc-daemon-mcp/internal/core"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type GetServerIdentityInput struct{}

type GetServerIdentityOutput = core.ServerIdentity

func RegisterIdentityTools(server *mcp.Server, service *core.Service) {
	mcp.AddTool(
		server,
		&mcp.Tool{
			Name:        "get_server_identity",
			Description: "Return relevant identity information reported by the local OpenSVC daemon.",
		},
		func(ctx context.Context, _ *mcp.CallToolRequest, _ GetServerIdentityInput) (*mcp.CallToolResult, GetServerIdentityOutput, error) {
			identity, err := service.GetServerIdentity(ctx)
			if err != nil {
				return nil, GetServerIdentityOutput{}, err
			}
			return nil, identity, nil
		},
	)
}
