package tools

import (
	"context"

	"github.com/hugobrenet/opensvc-daemon-mcp/internal/core"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type ListClusterObjectsInput struct {
	Selector string `json:"selector,omitempty" jsonschema:"optional OpenSVC object selector; defaults to **; examples: lab/**, */svc/*, lab/svc/redis"`
	Limit    int    `json:"limit,omitempty" jsonschema:"optional page size between 1 and 200; defaults to 100"`
	Cursor   string `json:"cursor,omitempty" jsonschema:"optional next_cursor returned by a previous call with the same selector"`
}

type ListClusterObjectsOutput = core.ClusterObjectList

func RegisterObjectTools(server *mcp.Server, service *core.Service) {
	mcp.AddTool(
		server,
		&mcp.Tool{
			Name:        "list_cluster_objects",
			Description: "List OpenSVC objects visible to the caller using a native selector and bounded cursor pagination.",
		},
		func(ctx context.Context, _ *mcp.CallToolRequest, input ListClusterObjectsInput) (*mcp.CallToolResult, ListClusterObjectsOutput, error) {
			objects, err := service.ListClusterObjects(ctx, core.ListClusterObjectsOptions{
				Selector: input.Selector,
				Limit:    input.Limit,
				Cursor:   input.Cursor,
			})
			if err != nil {
				return nil, ListClusterObjectsOutput{}, err
			}
			return nil, objects, nil
		},
	)
}
