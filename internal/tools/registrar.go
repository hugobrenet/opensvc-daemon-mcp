package tools

import (
	"fmt"
	"reflect"
	"regexp"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

const (
	maxToolNameBytes        = 128
	maxToolTitleBytes       = 256
	maxToolDescriptionBytes = 4 << 10
)

var toolNamePattern = regexp.MustCompile(`^[a-z][a-z0-9_]*$`)

type Registrar struct {
	server *mcp.Server
	names  map[string]struct{}
}

func NewRegistrar(server *mcp.Server) (*Registrar, error) {
	if server == nil {
		return nil, fmt.Errorf("tool registrar MCP server is nil")
	}
	return &Registrar{server: server, names: make(map[string]struct{})}, nil
}

func addTool[In, Out any](registrar *Registrar, tool *mcp.Tool, handler mcp.ToolHandlerFor[In, Out]) (err error) {
	if registrar == nil || registrar.server == nil {
		return fmt.Errorf("tool registrar is nil")
	}
	if err := validateToolDeclaration(tool); err != nil {
		return err
	}
	if handler == nil {
		return fmt.Errorf("tool %q handler is nil", tool.Name)
	}
	if !isObjectType(reflect.TypeFor[In]()) {
		return fmt.Errorf("tool %q input type must be a typed struct or map", tool.Name)
	}
	if !isObjectType(reflect.TypeFor[Out]()) {
		return fmt.Errorf("tool %q output type must be a typed struct or map", tool.Name)
	}
	if _, exists := registrar.names[tool.Name]; exists {
		return fmt.Errorf("tool name %q is already registered", tool.Name)
	}

	defer func() {
		if recovered := recover(); recovered != nil {
			err = fmt.Errorf("register tool %q: %v", tool.Name, recovered)
		}
	}()
	mcp.AddTool(registrar.server, tool, handler)
	registrar.names[tool.Name] = struct{}{}
	return nil
}

func isObjectType(valueType reflect.Type) bool {
	if valueType == nil {
		return false
	}
	return valueType.Kind() == reflect.Struct || valueType.Kind() == reflect.Map
}

func validateToolDeclaration(tool *mcp.Tool) error {
	if tool == nil {
		return fmt.Errorf("tool declaration is nil")
	}
	if !toolNamePattern.MatchString(tool.Name) {
		return fmt.Errorf("tool name %q must be lower snake_case", tool.Name)
	}
	if len(tool.Name) > maxToolNameBytes {
		return fmt.Errorf("tool name %q exceeds %d bytes", tool.Name, maxToolNameBytes)
	}
	if strings.TrimSpace(tool.Title) == "" {
		return fmt.Errorf("tool %q title is empty", tool.Name)
	}
	if len(tool.Title) > maxToolTitleBytes {
		return fmt.Errorf("tool %q title exceeds %d bytes", tool.Name, maxToolTitleBytes)
	}
	if strings.TrimSpace(tool.Description) == "" {
		return fmt.Errorf("tool %q description is empty", tool.Name)
	}
	if len(tool.Description) > maxToolDescriptionBytes {
		return fmt.Errorf("tool %q description exceeds %d bytes", tool.Name, maxToolDescriptionBytes)
	}
	if tool.Annotations == nil {
		return fmt.Errorf("tool %q annotations are missing", tool.Name)
	}
	if tool.Annotations.DestructiveHint == nil {
		return fmt.Errorf("tool %q destructive annotation is missing", tool.Name)
	}
	if *tool.Annotations.DestructiveHint {
		return fmt.Errorf("tool %q must be non-destructive", tool.Name)
	}
	if tool.Annotations.OpenWorldHint == nil {
		return fmt.Errorf("tool %q open-world annotation is missing", tool.Name)
	}
	if *tool.Annotations.OpenWorldHint {
		return fmt.Errorf("tool %q must be closed-world", tool.Name)
	}
	if len(tool.Meta) != 0 {
		return fmt.Errorf("tool %q custom metadata is unsupported", tool.Name)
	}
	return nil
}
