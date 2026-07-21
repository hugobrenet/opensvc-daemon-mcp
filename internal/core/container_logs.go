package core

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"unicode"
	"unicode/utf8"
)

const (
	defaultGetContainerLogsLines    = 50
	maxGetContainerLogsLines        = 200
	maxContainerLogContentRunes     = 64 << 10
	maxContainerLogRawBytes         = maxContainerLogContentRunes*utf8.UTFMax + 1
	maxContainerLogResourceIDLength = 255
)

var errContainerLogOutputLimit = errors.New("container log output limit reached")

type GetContainerLogsOptions struct {
	Path       string
	Node       string
	ResourceID string
	Lines      int
}

type ContainerLogs struct {
	Object     ClusterObjectReference `json:"object" jsonschema:"the canonical OpenSVC object reference"`
	Node       string                 `json:"node" jsonschema:"the exact node hosting the queried container resource"`
	ResourceID string                 `json:"resource_id" jsonschema:"the exact OpenSVC container resource id"`
	Lines      int                    `json:"lines" jsonschema:"the requested maximum number of recent container log records"`
	LineCount  int                    `json:"line_count" jsonschema:"the number of normalized text lines in content"`
	Content    string                 `json:"content" jsonschema:"the bounded recent container stdout and stderr log content"`
	Truncated  bool                   `json:"truncated" jsonschema:"whether log content was shortened by the MCP output bound"`
}

func (s *Service) GetContainerLogs(ctx context.Context, options GetContainerLogsOptions) (ContainerLogs, error) {
	reference, err := validateExactObjectPath(options.Path)
	if err != nil {
		return ContainerLogs{}, err
	}
	node, err := validateContainerLogNode(options.Node)
	if err != nil {
		return ContainerLogs{}, err
	}
	resourceID, err := validateContainerResourceID(options.ResourceID)
	if err != nil {
		return ContainerLogs{}, err
	}
	lines := options.Lines
	if lines == 0 {
		lines = defaultGetContainerLogsLines
	}
	if lines < 1 || lines > maxGetContainerLogsLines {
		return ContainerLogs{}, fmt.Errorf("container log lines must be between 1 and %d", maxGetContainerLogsLines)
	}
	getter, ok := s.client.(StreamGetter)
	if !ok {
		return ContainerLogs{}, fmt.Errorf("OpenSVC daemon client does not support stream requests")
	}

	endpoint := fmt.Sprintf(
		"/api/node/name/%s/instance/path/%s/%s/%s/container/log",
		node,
		reference.Namespace,
		reference.Kind,
		reference.Name,
	)
	query := url.Values{
		"rid":    {resourceID},
		"follow": {"false"},
		"lines":  {fmt.Sprintf("%d", lines)},
	}

	var raw bytes.Buffer
	rawTruncated := false
	err = getter.GetStream(ctx, endpoint, query, func(chunk []byte) error {
		remaining := maxContainerLogRawBytes - raw.Len()
		if remaining <= 0 {
			rawTruncated = true
			return errContainerLogOutputLimit
		}
		if len(chunk) > remaining {
			_, _ = raw.Write(chunk[:remaining])
			rawTruncated = true
			return errContainerLogOutputLimit
		}
		_, _ = raw.Write(chunk)
		return nil
	})
	if err != nil && !errors.Is(err, errContainerLogOutputLimit) {
		return ContainerLogs{}, fmt.Errorf("get container logs: %w", err)
	}

	content, contentTruncated := normalizeContainerLogContent(raw.Bytes())
	return ContainerLogs{
		Object:     reference,
		Node:       node,
		ResourceID: resourceID,
		Lines:      lines,
		LineCount:  containerLogLineCount(content),
		Content:    content,
		Truncated:  rawTruncated || contentTruncated,
	}, nil
}

func validateContainerLogNode(value string) (string, error) {
	node := strings.TrimSpace(value)
	if node == "" {
		return "", fmt.Errorf("container log node is required")
	}
	if len(node) > 255 {
		return "", fmt.Errorf("container log node exceeds 255 characters")
	}
	if node == "." || node == ".." || strings.ContainsAny(node, "/\\?#") {
		return "", fmt.Errorf("container log node %q is invalid", node)
	}
	return node, nil
}

func validateContainerResourceID(value string) (string, error) {
	resourceID := strings.TrimSpace(value)
	if resourceID == "" {
		return "", fmt.Errorf("container resource id is required")
	}
	if len(resourceID) > maxContainerLogResourceIDLength {
		return "", fmt.Errorf("container resource id exceeds %d characters", maxContainerLogResourceIDLength)
	}
	parts := strings.Split(resourceID, "#")
	if len(parts) != 2 || parts[0] != "container" || parts[1] == "" || strings.ContainsAny(parts[1], "*?[],/\\") {
		return "", fmt.Errorf("container resource id %q must identify one exact container# resource", resourceID)
	}
	return resourceID, nil
}

func normalizeContainerLogContent(raw []byte) (string, bool) {
	value := strings.ToValidUTF8(string(raw), "�")
	value = strings.ReplaceAll(value, "\r\n", "\n")
	value = strings.Map(func(r rune) rune {
		switch r {
		case '\n', '\t':
			return r
		case '\r':
			return '\n'
		default:
			if unicode.IsControl(r) || unicode.In(r, unicode.Cf) {
				return ' '
			}
			return r
		}
	}, value)
	runes := []rune(value)
	truncated := len(runes) > maxContainerLogContentRunes
	if truncated {
		value = string(runes[:maxContainerLogContentRunes-1]) + "…"
	}
	return strings.TrimSpace(value), truncated
}

func containerLogLineCount(content string) int {
	if content == "" {
		return 0
	}
	return strings.Count(content, "\n") + 1
}
