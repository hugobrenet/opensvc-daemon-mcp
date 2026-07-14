package tools

import (
	"context"

	"github.com/hugobrenet/opensvc-daemon-mcp/internal/core"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type GetDaemonIdentityInput struct{}

type GetDaemonIdentityOutput = core.DaemonIdentity

func RegisterDaemonTools(server *mcp.Server, service *core.Service) {
	mcp.AddTool(
		server,
		&mcp.Tool{
			Name:        "get_daemon_identity",
			Description: "Return relevant identity information reported by the local OpenSVC daemon.",
		},
		func(ctx context.Context, _ *mcp.CallToolRequest, _ GetDaemonIdentityInput) (*mcp.CallToolResult, GetDaemonIdentityOutput, error) {
			identity, err := service.GetDaemonIdentity(ctx)
			if err != nil {
				return nil, GetDaemonIdentityOutput{}, err
			}
			return nil, identity, nil
		},
	)
}
