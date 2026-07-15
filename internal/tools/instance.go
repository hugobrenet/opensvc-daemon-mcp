package tools

import (
	"context"
	"time"

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

type RefreshInstanceStatusInput struct {
	Path           string `json:"path" jsonschema:"the exact canonical OpenSVC object path returned by list_cluster_objects"`
	Node           string `json:"node" jsonschema:"the exact node name hosting the instance to refresh"`
	TimeoutSeconds int    `json:"timeout_seconds,omitempty" jsonschema:"optional polling timeout in seconds between 5 and 120; defaults to 30"`
}

type RefreshInstanceStatusOutput = core.RefreshInstanceStatusResult

type GetInstanceLogsInput struct {
	Path  string `json:"path" jsonschema:"the exact canonical OpenSVC object path returned by list_cluster_objects"`
	Node  string `json:"node" jsonschema:"the exact node name hosting the instance whose OpenSVC logs are requested"`
	Lines int    `json:"lines,omitempty" jsonschema:"optional maximum number of recent log entries between 1 and 100; defaults to 50"`
}

type GetInstanceLogsOutput = core.InstanceLogList

func RegisterInstanceTools(server *mcp.Server, service *core.Service) {
	mcp.AddTool(
		server,
		&mcp.Tool{
			Name:        "get_instance_logs",
			Title:       "Get instance logs",
			Description: "Read bounded recent OpenSVC orchestration and daemon logs for one exact object instance. This finite read does not follow the stream and does not return workload stdout or stderr; the daemon currently requires root access for this endpoint.",
			Annotations: readOnlyClosedWorldAnnotations(),
		},
		func(ctx context.Context, _ *mcp.CallToolRequest, input GetInstanceLogsInput) (*mcp.CallToolResult, GetInstanceLogsOutput, error) {
			logs, err := service.GetInstanceLogs(ctx, core.GetInstanceLogsOptions{
				Path:  input.Path,
				Node:  input.Node,
				Lines: input.Lines,
			})
			if err != nil {
				return nil, GetInstanceLogsOutput{}, err
			}
			return nil, logs, nil
		},
	)

	mcp.AddTool(
		server,
		&mcp.Tool{
			Name:        "refresh_instance_status",
			Title:       "Refresh instance status",
			Description: "Actively run an OpenSVC status probe for one exact object instance, then poll until a newer status is observed or the bounded timeout expires. Requires operator access on the namespace; it is non-destructive but executes resource drivers and updates daemon state.",
			Annotations: activeNonDestructiveClosedWorldAnnotations(),
		},
		func(ctx context.Context, _ *mcp.CallToolRequest, input RefreshInstanceStatusInput) (*mcp.CallToolResult, RefreshInstanceStatusOutput, error) {
			result, err := service.RefreshInstanceStatus(ctx, core.RefreshInstanceStatusOptions{
				Path:    input.Path,
				Node:    input.Node,
				Timeout: time.Duration(input.TimeoutSeconds) * time.Second,
			})
			if err != nil {
				return nil, RefreshInstanceStatusOutput{}, err
			}
			return nil, result, nil
		},
	)

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
