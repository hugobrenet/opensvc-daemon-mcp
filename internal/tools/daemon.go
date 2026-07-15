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
			Name:  "get_daemon_identity",
			Title: "Get daemon identity",
			Description: "Inspect identity and compatibility metadata for the local OpenSVC daemon, node, and cluster. " +
				"Use this first to confirm the target environment; it returns no object configuration and makes no changes.",
			Annotations: readOnlyClosedWorldAnnotations(),
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
