package tools

import (
	"context"

	"github.com/hugobrenet/opensvc-daemon-mcp/internal/core"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type ListObjectResourcesInput struct {
	Path   string `json:"path" jsonschema:"the exact canonical OpenSVC object path returned by list_cluster_objects"`
	Node   string `json:"node,omitempty" jsonschema:"optional exact OpenSVC node name used to filter resources"`
	RID    string `json:"rid,omitempty" jsonschema:"optional OpenSVC resource id or resource match expression"`
	Limit  int    `json:"limit,omitempty" jsonschema:"optional page size between 1 and 200; defaults to 100"`
	Cursor string `json:"cursor,omitempty" jsonschema:"optional opaque next_cursor returned by a previous call with the same filters"`
}

type ListObjectResourcesOutput = core.ObjectResourceList

func RegisterResourceTools(server *mcp.Server, service *core.Service) {
	mcp.AddTool(
		server,
		&mcp.Tool{
			Name:        "list_object_resources",
			Title:       "List object resources",
			Description: "Inspect the last-known resource status, provisioning, monitoring flags, restart state, and bounded status messages for one exact OpenSVC object. This read-only call does not probe drivers; filter by node or resource id after checking the parent instance updated_at.",
			Annotations: readOnlyClosedWorldAnnotations(),
		},
		func(ctx context.Context, _ *mcp.CallToolRequest, input ListObjectResourcesInput) (*mcp.CallToolResult, ListObjectResourcesOutput, error) {
			resources, err := service.ListObjectResources(ctx, core.ListObjectResourcesOptions{
				Path:   input.Path,
				Node:   input.Node,
				RID:    input.RID,
				Limit:  input.Limit,
				Cursor: input.Cursor,
			})
			if err != nil {
				return nil, ListObjectResourcesOutput{}, err
			}
			return nil, resources, nil
		},
	)
}
