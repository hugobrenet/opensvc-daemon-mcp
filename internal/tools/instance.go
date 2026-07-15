package tools

import (
	"context"

	"github.com/hugobrenet/opensvc-daemon-mcp/internal/core"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type ListObjectInstancesInput struct {
	Path   string `json:"path" jsonschema:"the exact canonical OpenSVC object path returned by list_cluster_objects"`
	Node   string `json:"node,omitempty" jsonschema:"optional exact OpenSVC node name used to filter instances"`
	Limit  int    `json:"limit,omitempty" jsonschema:"optional page size between 1 and 100; defaults to 50"`
	Cursor string `json:"cursor,omitempty" jsonschema:"optional next_cursor returned by a previous call with the same path and node filter"`
}

type ListObjectInstancesOutput = core.ObjectInstanceList

func RegisterInstanceTools(server *mcp.Server, service *core.Service) {
	mcp.AddTool(
		server,
		&mcp.Tool{
			Name:        "list_object_instances",
			Title:       "List object instances",
			Description: "Inspect the last-known monitor state, target state, availability, and resource status counts for the instances of one exact OpenSVC object. This read-only call does not run a status refresh; compare updated_at with the current time before diagnosing a node instance.",
			Annotations: readOnlyClosedWorldAnnotations(),
		},
		func(ctx context.Context, _ *mcp.CallToolRequest, input ListObjectInstancesInput) (*mcp.CallToolResult, ListObjectInstancesOutput, error) {
			instances, err := service.ListObjectInstances(ctx, core.ListObjectInstancesOptions{
				Path:   input.Path,
				Node:   input.Node,
				Limit:  input.Limit,
				Cursor: input.Cursor,
			})
			if err != nil {
				return nil, ListObjectInstancesOutput{}, err
			}
			return nil, instances, nil
		},
	)
}
