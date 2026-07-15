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

type GetObjectStatusInput struct {
	Path string `json:"path" jsonschema:"the exact canonical OpenSVC object path returned by list_cluster_objects"`
}

type GetObjectStatusOutput = core.ObjectStatus

func RegisterObjectTools(server *mcp.Server, service *core.Service) {
	mcp.AddTool(
		server,
		&mcp.Tool{
			Name:        "get_object_status",
			Title:       "Get object status",
			Description: "Inspect the last-known aggregate operational status and instance placement of one exact OpenSVC object. This read-only call does not refresh instance drivers; use updated_at to assess freshness before drilling into instance or resource details.",
			Annotations: readOnlyClosedWorldAnnotations(),
		},
		func(ctx context.Context, _ *mcp.CallToolRequest, input GetObjectStatusInput) (*mcp.CallToolResult, GetObjectStatusOutput, error) {
			status, err := service.GetObjectStatus(ctx, input.Path)
			if err != nil {
				return nil, GetObjectStatusOutput{}, err
			}
			return nil, status, nil
		},
	)

	mcp.AddTool(
		server,
		&mcp.Tool{
			Name:  "list_cluster_objects",
			Title: "List cluster objects",
			Description: "Discover OpenSVC object paths visible to the delegated caller using a native selector and bounded cursor pagination. " +
				"Use returned paths as identifiers for object-specific tools; no status or configuration is returned.",
			Annotations: readOnlyClosedWorldAnnotations(),
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
