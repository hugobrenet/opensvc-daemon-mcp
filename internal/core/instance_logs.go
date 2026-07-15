package core

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"slices"
	"strings"
	"unicode"
)

const (
	defaultGetInstanceLogsLines      = 50
	maxGetInstanceLogsLines          = 100
	maxInstanceLogMessageRunes       = 2048
	maxInstanceLogsTotalMessageRunes = 64 << 10
	maxInstanceLogFieldRunes         = 255
)

type GetInstanceLogsOptions struct {
	Path  string
	Node  string
	Lines int
}

type InstanceLogList struct {
	Object    ClusterObjectReference `json:"object" jsonschema:"the canonical OpenSVC object reference"`
	Node      string                 `json:"node" jsonschema:"the exact node hosting the queried object instance"`
	Lines     int                    `json:"lines" jsonschema:"the requested maximum number of recent log entries"`
	Count     int                    `json:"count" jsonschema:"the number of bounded log entries returned"`
	Entries   []InstanceLogEntry     `json:"entries" jsonschema:"the recent OpenSVC instance log entries in chronological order"`
	Truncated bool                   `json:"truncated" jsonschema:"whether older entries or message content were omitted by response bounds"`
}

type InstanceLogEntry struct {
	Timestamp        string `json:"timestamp" jsonschema:"the timestamp reported by the OpenSVC log payload"`
	Level            string `json:"level" jsonschema:"the normalized OpenSVC log level"`
	Message          string `json:"message" jsonschema:"the bounded OpenSVC log message without raw journald metadata"`
	MessageTruncated bool   `json:"message_truncated" jsonschema:"whether this log message was shortened by MCP output bounds"`
	Component        string `json:"component,omitempty" jsonschema:"the OpenSVC package or component that emitted the log entry"`
	ResourceID       string `json:"resource_id,omitempty" jsonschema:"the related OpenSVC resource id when present"`
	SessionID        string `json:"session_id,omitempty" jsonschema:"the related OpenSVC session id when present"`
	EventID          string `json:"event_id,omitempty" jsonschema:"the related OpenSVC event id when present"`
	RequestID        string `json:"request_id,omitempty" jsonschema:"the related daemon API request id when present"`
	OrchestrationID  string `json:"orchestration_id,omitempty" jsonschema:"the related OpenSVC orchestration id when present"`
}

type daemonInstanceLogEnvelope struct {
	JSON            string `json:"JSON"`
	Timestamp       string `json:"__REALTIME_TIMESTAMP"`
	Level           string `json:"LEVEL"`
	Message         string `json:"MESSAGE"`
	Node            string `json:"NODE"`
	Object          string `json:"OBJ_PATH"`
	Component       string `json:"PKG"`
	ResourceID      string `json:"RID"`
	SessionID       string `json:"SID"`
	EventID         string `json:"EID"`
	RequestID       string `json:"REQUEST_UUID"`
	OrchestrationID string `json:"ORCHESTRATION_ID"`
}

type daemonInstanceLogPayload struct {
	Timestamp       string `json:"time"`
	Level           string `json:"level"`
	Message         string `json:"message"`
	Node            string `json:"node"`
	Object          string `json:"obj_path"`
	Component       string `json:"pkg"`
	ResourceID      string `json:"rid"`
	SessionID       string `json:"sid"`
	EventID         string `json:"eid"`
	RequestID       string `json:"request_uuid"`
	OrchestrationID string `json:"orchestration_id"`
}

func (s *Service) GetInstanceLogs(ctx context.Context, options GetInstanceLogsOptions) (InstanceLogList, error) {
	reference, err := validateExactObjectPath(options.Path)
	if err != nil {
		return InstanceLogList{}, err
	}
	node := strings.TrimSpace(options.Node)
	if node == "" {
		return InstanceLogList{}, fmt.Errorf("instance log node is required")
	}
	if len(node) > 255 {
		return InstanceLogList{}, fmt.Errorf("instance log node exceeds 255 characters")
	}
	lines := options.Lines
	if lines == 0 {
		lines = defaultGetInstanceLogsLines
	}
	if lines < 1 || lines > maxGetInstanceLogsLines {
		return InstanceLogList{}, fmt.Errorf("instance log lines must be between 1 and %d", maxGetInstanceLogsLines)
	}
	getter, ok := s.client.(SSEGetter)
	if !ok {
		return InstanceLogList{}, fmt.Errorf("OpenSVC daemon client does not support SSE requests")
	}

	endpoint := fmt.Sprintf(
		"/api/node/name/%s/instance/path/%s/%s/%s/log",
		node,
		reference.Namespace,
		reference.Kind,
		reference.Name,
	)
	query := url.Values{
		"follow": {"false"},
		"lines":  {fmt.Sprintf("%d", lines+1)},
	}
	entries := make([]InstanceLogEntry, 0, lines+1)
	err = getter.GetSSE(ctx, endpoint, query, func(event string, _ string, data []byte) error {
		if event != "" && event != "log" {
			return fmt.Errorf("unexpected instance log SSE event %q", event)
		}
		entry, err := parseInstanceLogEntry(data, reference.Path, node)
		if err != nil {
			return err
		}
		entries = append(entries, entry)
		if len(entries) > lines+1 {
			return fmt.Errorf("instance log endpoint returned more than %d requested events", lines+1)
		}
		return nil
	})
	if err != nil {
		return InstanceLogList{}, fmt.Errorf("get instance logs: %w", err)
	}

	truncated := len(entries) > lines
	if truncated {
		entries = entries[len(entries)-lines:]
	}
	entries, bounded := boundInstanceLogEntries(entries)
	truncated = truncated || bounded
	if entries == nil {
		entries = []InstanceLogEntry{}
	}
	return InstanceLogList{
		Object: reference, Node: node, Lines: lines, Count: len(entries),
		Entries: entries, Truncated: truncated,
	}, nil
}

func parseInstanceLogEntry(data []byte, expectedObject string, expectedNode string) (InstanceLogEntry, error) {
	var envelope daemonInstanceLogEnvelope
	if err := json.Unmarshal(data, &envelope); err != nil {
		return InstanceLogEntry{}, fmt.Errorf("decode instance log envelope: %w", err)
	}
	payload := daemonInstanceLogPayload{
		Timestamp: envelope.Timestamp, Level: envelope.Level, Message: envelope.Message,
		Node: envelope.Node, Object: envelope.Object, Component: envelope.Component,
		ResourceID: envelope.ResourceID, SessionID: envelope.SessionID, EventID: envelope.EventID,
		RequestID: envelope.RequestID, OrchestrationID: envelope.OrchestrationID,
	}
	if envelope.JSON != "" {
		var nested daemonInstanceLogPayload
		if err := json.Unmarshal([]byte(envelope.JSON), &nested); err != nil {
			return InstanceLogEntry{}, fmt.Errorf("decode nested OpenSVC instance log payload: %w", err)
		}
		mergeInstanceLogPayload(&payload, nested)
	}
	if payload.Object != "" && payload.Object != expectedObject {
		return InstanceLogEntry{}, fmt.Errorf("instance log returned unexpected object %q", payload.Object)
	}
	if payload.Node != "" && payload.Node != expectedNode {
		return InstanceLogEntry{}, fmt.Errorf("instance log returned unexpected node %q", payload.Node)
	}
	payload.Message = normalizeInstanceLogText(payload.Message)
	if payload.Message == "" {
		return InstanceLogEntry{}, fmt.Errorf("instance log entry has no message")
	}
	return InstanceLogEntry{
		Timestamp:       boundInstanceLogField(payload.Timestamp),
		Level:           strings.ToLower(boundInstanceLogField(payload.Level)),
		Message:         payload.Message,
		Component:       boundInstanceLogField(payload.Component),
		ResourceID:      boundInstanceLogField(payload.ResourceID),
		SessionID:       boundInstanceLogField(payload.SessionID),
		EventID:         boundInstanceLogField(payload.EventID),
		RequestID:       boundInstanceLogField(payload.RequestID),
		OrchestrationID: boundInstanceLogField(payload.OrchestrationID),
	}, nil
}

func mergeInstanceLogPayload(target *daemonInstanceLogPayload, source daemonInstanceLogPayload) {
	if source.Timestamp != "" {
		target.Timestamp = source.Timestamp
	}
	if source.Level != "" {
		target.Level = source.Level
	}
	if source.Message != "" {
		target.Message = source.Message
	}
	if source.Node != "" {
		target.Node = source.Node
	}
	if source.Object != "" {
		target.Object = source.Object
	}
	if source.Component != "" {
		target.Component = source.Component
	}
	if source.ResourceID != "" {
		target.ResourceID = source.ResourceID
	}
	if source.SessionID != "" {
		target.SessionID = source.SessionID
	}
	if source.EventID != "" {
		target.EventID = source.EventID
	}
	if source.RequestID != "" {
		target.RequestID = source.RequestID
	}
	if source.OrchestrationID != "" {
		target.OrchestrationID = source.OrchestrationID
	}
}

func boundInstanceLogEntries(entries []InstanceLogEntry) ([]InstanceLogEntry, bool) {
	remaining := maxInstanceLogsTotalMessageRunes
	selected := make([]InstanceLogEntry, 0, len(entries))
	truncated := false
	for index := len(entries) - 1; index >= 0; index-- {
		if remaining == 0 {
			truncated = true
			break
		}
		entry := entries[index]
		message, messageTruncated, used := boundInstanceLogMessage(entry.Message, remaining)
		entry.Message = message
		entry.MessageTruncated = messageTruncated
		remaining -= used
		truncated = truncated || messageTruncated
		selected = append(selected, entry)
	}
	slices.Reverse(selected)
	return selected, truncated
}

func boundInstanceLogMessage(message string, remaining int) (string, bool, int) {
	runes := []rune(message)
	allowed := min(len(runes), maxInstanceLogMessageRunes, max(remaining, 0))
	if allowed == len(runes) {
		return message, false, allowed
	}
	if allowed == 0 {
		return "", len(runes) > 0, 0
	}
	return string(runes[:allowed-1]) + "…", true, allowed
}

func boundInstanceLogField(value string) string {
	value = normalizeInstanceLogText(value)
	runes := []rune(value)
	if len(runes) <= maxInstanceLogFieldRunes {
		return value
	}
	return string(runes[:maxInstanceLogFieldRunes-1]) + "…"
}

func normalizeInstanceLogText(value string) string {
	value = strings.Map(func(r rune) rune {
		if unicode.IsControl(r) || unicode.In(r, unicode.Cf) {
			return ' '
		}
		return r
	}, value)
	return strings.Join(strings.Fields(value), " ")
}
