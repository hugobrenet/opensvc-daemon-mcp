package tools

import (
	"context"

	"github.com/hugobrenet/opensvc-daemon-mcp/internal/core"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type GetClusterHealthInput struct{}

type GetClusterHealthOutput = core.ClusterHealth

func RegisterClusterTools(server *mcp.Server, service *core.Service) {
	mcp.AddTool(
		server,
		&mcp.Tool{
			Name:        "get_cluster_health",
			Description: "Return a deterministic health assessment of the OpenSVC cluster, its nodes, and actor objects.",
		},
		func(ctx context.Context, _ *mcp.CallToolRequest, _ GetClusterHealthInput) (*mcp.CallToolResult, GetClusterHealthOutput, error) {
			health, err := service.GetClusterHealth(ctx)
			if err != nil {
				return nil, GetClusterHealthOutput{}, err
			}
			return nil, health, nil
		},
	)
}
