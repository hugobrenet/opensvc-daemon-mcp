package core

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"reflect"
	"strings"
	"testing"
)

type containerLogsClient struct {
	t      *testing.T
	path   string
	query  url.Values
	chunks [][]byte
	err    error
	calls  int
}

func (f *containerLogsClient) GetJSON(context.Context, string, url.Values, any) error {
	f.t.Helper()
	return fmt.Errorf("unexpected JSON request")
}

func (f *containerLogsClient) GetStream(_ context.Context, path string, query url.Values, consume func([]byte) error) error {
	f.t.Helper()
	f.calls++
	if path != f.path {
		return fmt.Errorf("got stream path %q, want %q", path, f.path)
	}
	if !reflect.DeepEqual(query, f.query) {
		return fmt.Errorf("got stream query %#v, want %#v", query, f.query)
	}
	if f.err != nil {
		return f.err
	}
	for _, chunk := range f.chunks {
		if err := consume(chunk); err != nil {
			return err
		}
	}
	return nil
}

func TestGetContainerLogs(t *testing.T) {
	client := &containerLogsClient{
		t:    t,
		path: "/api/node/name/node-a/instance/path/lab/svc/redis/container/log",
		query: url.Values{
			"rid": {"container#redis"}, "follow": {"false"}, "lines": {"50"},
		},
		chunks: [][]byte{
			[]byte("Redis starting\r\n"),
			append([]byte("Ready\x00 "), 0xff),
		},
	}

	result, err := New(client).GetContainerLogs(context.Background(), GetContainerLogsOptions{
		Path: " lab/svc/redis ", Node: " node-a ", ResourceID: " container#redis ",
	})
	if err != nil {
		t.Fatalf("get container logs: %v", err)
	}
	if client.calls != 1 || result.Object.Path != "lab/svc/redis" || result.Node != "node-a" || result.ResourceID != "container#redis" || result.Lines != defaultGetContainerLogsLines {
		t.Fatalf("got unexpected container logs %+v client=%+v", result, client)
	}
	if result.Content != "Redis starting\nReady  �" || result.LineCount != 2 || result.Truncated {
		t.Errorf("got unexpected normalized content %+v", result)
	}
}

func TestGetContainerLogsBoundsContent(t *testing.T) {
	client := &containerLogsClient{
		t:    t,
		path: "/api/node/name/node-a/instance/path/lab/svc/redis/container/log",
		query: url.Values{
			"rid": {"container#redis"}, "follow": {"false"}, "lines": {"1"},
		},
		chunks: [][]byte{[]byte(strings.Repeat("é", maxContainerLogContentRunes+1))},
	}

	result, err := New(client).GetContainerLogs(context.Background(), GetContainerLogsOptions{
		Path: "lab/svc/redis", Node: "node-a", ResourceID: "container#redis", Lines: 1,
	})
	if err != nil {
		t.Fatalf("get container logs: %v", err)
	}
	runes := []rune(result.Content)
	if !result.Truncated || len(runes) != maxContainerLogContentRunes || runes[len(runes)-1] != '…' {
		t.Errorf("container log content was not bounded: length=%d result=%+v", len(runes), result)
	}
}

func TestGetContainerLogsRejectsInvalidInputBeforeDaemonCall(t *testing.T) {
	for _, test := range []struct {
		name    string
		options GetContainerLogsOptions
	}{
		{name: "node", options: GetContainerLogsOptions{Path: "lab/svc/redis", ResourceID: "container#redis"}},
		{name: "node path", options: GetContainerLogsOptions{Path: "lab/svc/redis", Node: "../node-a", ResourceID: "container#redis"}},
		{name: "resource", options: GetContainerLogsOptions{Path: "lab/svc/redis", Node: "node-a"}},
		{name: "resource group", options: GetContainerLogsOptions{Path: "lab/svc/redis", Node: "node-a", ResourceID: "disk#data"}},
		{name: "resource selector", options: GetContainerLogsOptions{Path: "lab/svc/redis", Node: "node-a", ResourceID: "container#*"}},
		{name: "lines", options: GetContainerLogsOptions{Path: "lab/svc/redis", Node: "node-a", ResourceID: "container#redis", Lines: maxGetContainerLogsLines + 1}},
	} {
		t.Run(test.name, func(t *testing.T) {
			client := &containerLogsClient{t: t}
			if _, err := New(client).GetContainerLogs(context.Background(), test.options); err == nil {
				t.Fatal("GetContainerLogs succeeded, want an error")
			}
			if client.calls != 0 {
				t.Fatalf("got %d daemon calls, want 0", client.calls)
			}
		})
	}
}

func TestGetContainerLogsPropagatesStreamError(t *testing.T) {
	want := errors.New("stream failed")
	client := &containerLogsClient{
		t:    t,
		path: "/api/node/name/node-a/instance/path/lab/svc/redis/container/log",
		query: url.Values{
			"rid": {"container#redis"}, "follow": {"false"}, "lines": {"50"},
		},
		err: want,
	}
	_, err := New(client).GetContainerLogs(context.Background(), GetContainerLogsOptions{
		Path: "lab/svc/redis", Node: "node-a", ResourceID: "container#redis",
	})
	if !errors.Is(err, want) {
		t.Fatalf("got error %v, want stream error", err)
	}
}
