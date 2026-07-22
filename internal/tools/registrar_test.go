package tools

import (
	"context"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type registrarTestInput struct{}
type registrarTestOutput struct {
	Status string `json:"status" jsonschema:"test status"`
}

func TestValidateToolDeclarationRejectsInvalidMetadata(t *testing.T) {
	trueValue := true
	falseValue := false
	for _, test := range []struct {
		name string
		tool *mcp.Tool
	}{
		{name: "nil tool"},
		{name: "invalid name", tool: validTestTool("Invalid-Name")},
		{name: "long name", tool: validTestTool("a" + strings.Repeat("b", maxToolNameBytes))},
		{name: "empty title", tool: mutateTestTool(func(tool *mcp.Tool) { tool.Title = " " })},
		{name: "long title", tool: mutateTestTool(func(tool *mcp.Tool) { tool.Title = strings.Repeat("t", maxToolTitleBytes+1) })},
		{name: "empty description", tool: mutateTestTool(func(tool *mcp.Tool) { tool.Description = " " })},
		{name: "long description", tool: mutateTestTool(func(tool *mcp.Tool) { tool.Description = strings.Repeat("d", maxToolDescriptionBytes+1) })},
		{name: "missing annotations", tool: mutateTestTool(func(tool *mcp.Tool) { tool.Annotations = nil })},
		{name: "missing destructive", tool: mutateTestTool(func(tool *mcp.Tool) { tool.Annotations.DestructiveHint = nil })},
		{name: "destructive", tool: mutateTestTool(func(tool *mcp.Tool) { tool.Annotations.DestructiveHint = &trueValue })},
		{name: "missing open world", tool: mutateTestTool(func(tool *mcp.Tool) { tool.Annotations.OpenWorldHint = nil })},
		{name: "open world", tool: mutateTestTool(func(tool *mcp.Tool) { tool.Annotations.OpenWorldHint = &trueValue })},
		{name: "custom metadata", tool: mutateTestTool(func(tool *mcp.Tool) { tool.Meta = mcp.Meta{"tag": "diagnostic"} })},
		{name: "explicit false values are valid", tool: mutateTestTool(func(tool *mcp.Tool) {
			tool.Annotations.DestructiveHint = &falseValue
			tool.Annotations.OpenWorldHint = &falseValue
		})},
	} {
		t.Run(test.name, func(t *testing.T) {
			err := validateToolDeclaration(test.tool)
			if test.name == "explicit false values are valid" {
				if err != nil {
					t.Fatalf("valid declaration error: %v", err)
				}
				return
			}
			if err == nil {
				t.Fatal("invalid declaration succeeded")
			}
		})
	}
}

func TestRegistrarRejectsDuplicateNames(t *testing.T) {
	registrar := newTestRegistrar(t)
	if err := addTool(registrar, validTestTool("test_tool"), registrarTestHandler); err != nil {
		t.Fatalf("register first tool: %v", err)
	}
	if err := addTool(registrar, validTestTool("test_tool"), registrarTestHandler); err == nil || !strings.Contains(err.Error(), "already registered") {
		t.Fatalf("duplicate registration error = %v", err)
	}
}

func TestRegistrarRejectsInvalidRegistration(t *testing.T) {
	registrar := newTestRegistrar(t)

	if err := addTool[registrarTestInput, registrarTestOutput](nil, validTestTool("nil_registrar"), registrarTestHandler); err == nil {
		t.Fatal("nil registrar succeeded")
	}
	if err := addTool(registrar, validTestTool("nil_handler"), mcp.ToolHandlerFor[registrarTestInput, registrarTestOutput](nil)); err == nil {
		t.Fatal("nil handler succeeded")
	}
	if err := addTool(registrar, validTestTool("untyped_input"), registrarUntypedInputHandler); err == nil || !strings.Contains(err.Error(), "input type") {
		t.Fatalf("untyped input error = %v", err)
	}
	if err := addTool(registrar, validTestTool("untyped_output"), registrarUntypedOutputHandler); err == nil || !strings.Contains(err.Error(), "output type") {
		t.Fatalf("untyped output error = %v", err)
	}
}

func TestRegisterAllToolDomains(t *testing.T) {
	registrar := newTestRegistrar(t)
	for name, register := range map[string]func(*Registrar) error{
		"daemon":   func(registrar *Registrar) error { return RegisterDaemonTools(registrar, nil) },
		"cluster":  func(registrar *Registrar) error { return RegisterClusterTools(registrar, nil) },
		"object":   func(registrar *Registrar) error { return RegisterObjectTools(registrar, nil) },
		"instance": func(registrar *Registrar) error { return RegisterInstanceTools(registrar, nil) },
		"resource": func(registrar *Registrar) error { return RegisterResourceTools(registrar, nil) },
	} {
		t.Run(name, func(t *testing.T) {
			if err := register(registrar); err != nil {
				t.Fatalf("register tools: %v", err)
			}
		})
	}
}

func TestRegistrarConvertsSDKSchemaPanicToError(t *testing.T) {
	registrar := newTestRegistrar(t)
	tool := validTestTool("invalid_schema")
	tool.InputSchema = map[string]any{"type": "string"}
	if err := addTool(registrar, tool, registrarTestHandler); err == nil || !strings.Contains(err.Error(), "input schema") {
		t.Fatalf("invalid schema error = %v", err)
	}
}

func TestNewRegistrarRejectsNilServer(t *testing.T) {
	if _, err := NewRegistrar(nil); err == nil {
		t.Fatal("nil MCP server succeeded")
	}
}

func newTestRegistrar(t *testing.T) *Registrar {
	t.Helper()
	server := mcp.NewServer(&mcp.Implementation{Name: "test", Version: "test"}, nil)
	registrar, err := NewRegistrar(server)
	if err != nil {
		t.Fatalf("create registrar: %v", err)
	}
	return registrar
}

func validTestTool(name string) *mcp.Tool {
	return &mcp.Tool{
		Name:        name,
		Title:       "Test tool",
		Description: "A complete test tool declaration.",
		Annotations: readOnlyClosedWorldAnnotations(),
	}
}

func mutateTestTool(mutate func(*mcp.Tool)) *mcp.Tool {
	tool := validTestTool("test_tool")
	mutate(tool)
	return tool
}

func registrarTestHandler(context.Context, *mcp.CallToolRequest, registrarTestInput) (*mcp.CallToolResult, registrarTestOutput, error) {
	return nil, registrarTestOutput{Status: "ok"}, nil
}

func registrarUntypedInputHandler(context.Context, *mcp.CallToolRequest, any) (*mcp.CallToolResult, registrarTestOutput, error) {
	return nil, registrarTestOutput{Status: "ok"}, nil
}

func registrarUntypedOutputHandler(context.Context, *mcp.CallToolRequest, registrarTestInput) (*mcp.CallToolResult, any, error) {
	return nil, registrarTestOutput{Status: "ok"}, nil
}
