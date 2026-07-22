package tools

import (
	"context"

	"github.com/hugobrenet/opensvc-daemon-mcp/internal/core"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type GetClusterHealthInput struct{}

type GetClusterHealthOutput = core.ClusterHealth

func RegisterClusterTools(registrar *Registrar, service *core.Service) error {
	if err := addTool(
		registrar,
		&mcp.Tool{
			Name:  "get_cluster_health",
			Title: "Assess cluster health",
			Description: "Compute a deterministic health assessment from the last-known OpenSVC cluster status for the cluster, nodes, and visible actor objects. " +
				"This read-only call does not refresh instance drivers; healthy means no problem in the status currently published by the daemon, not a real-time probe.",
			Annotations: readOnlyClosedWorldAnnotations(),
		},
		func(ctx context.Context, _ *mcp.CallToolRequest, _ GetClusterHealthInput) (*mcp.CallToolResult, GetClusterHealthOutput, error) {
			health, err := service.GetClusterHealth(ctx)
			if err != nil {
				return nil, GetClusterHealthOutput{}, err
			}
			return nil, health, nil
		},
	); err != nil {
		return err
	}
	return nil
}
