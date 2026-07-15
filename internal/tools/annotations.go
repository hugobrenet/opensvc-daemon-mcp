package tools

import "github.com/modelcontextprotocol/go-sdk/mcp"

func readOnlyClosedWorldAnnotations() *mcp.ToolAnnotations {
	destructive := false
	openWorld := false
	return &mcp.ToolAnnotations{
		DestructiveHint: &destructive,
		OpenWorldHint:   &openWorld,
		ReadOnlyHint:    true,
	}
}

func activeNonDestructiveClosedWorldAnnotations() *mcp.ToolAnnotations {
	destructive := false
	openWorld := false
	return &mcp.ToolAnnotations{
		DestructiveHint: &destructive,
		IdempotentHint:  false,
		OpenWorldHint:   &openWorld,
		ReadOnlyHint:    false,
	}
}
