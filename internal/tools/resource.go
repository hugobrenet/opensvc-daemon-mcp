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

type GetContainerLogsInput struct {
	Path       string `json:"path" jsonschema:"the exact canonical OpenSVC object path returned by list_cluster_objects"`
	Node       string `json:"node" jsonschema:"the exact node hosting the container resource"`
	ResourceID string `json:"resource_id" jsonschema:"the exact OpenSVC container resource id returned by list_object_resources; example: container#redis"`
	Lines      int    `json:"lines,omitempty" jsonschema:"optional maximum recent container log records requested from OpenSVC between 1 and 200; defaults to 50"`
}

type GetContainerLogsOutput = core.ContainerLogs

func RegisterResourceTools(registrar *Registrar, service *core.Service) error {
	if err := addTool(
		registrar,
		&mcp.Tool{
			Name:        "get_container_logs",
			Title:       "Get container logs",
			Description: "Read bounded recent stdout and stderr logs for one exact OpenSVC container resource. This finite read never follows the stream; application logs may contain sensitive data, and the daemon currently requires root access for this endpoint.",
			Annotations: readOnlyClosedWorldAnnotations(),
		},
		func(ctx context.Context, _ *mcp.CallToolRequest, input GetContainerLogsInput) (*mcp.CallToolResult, GetContainerLogsOutput, error) {
			logs, err := service.GetContainerLogs(ctx, core.GetContainerLogsOptions{
				Path:       input.Path,
				Node:       input.Node,
				ResourceID: input.ResourceID,
				Lines:      input.Lines,
			})
			if err != nil {
				return nil, GetContainerLogsOutput{}, err
			}
			return nil, logs, nil
		},
	); err != nil {
		return err
	}

	if err := addTool(
		registrar,
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
	); err != nil {
		return err
	}
	return nil
}
