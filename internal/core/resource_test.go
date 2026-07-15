package core

import (
	"context"
	"net/url"
	"testing"
)

func TestListObjectResources(t *testing.T) {
	client := &recordingJSONGetter{
		t:     t,
		path:  "/api/resource",
		query: url.Values{"path": {"prod/svc/mysql"}, "node": {"node-a"}},
		payload: `{
			"kind": "ResourceList",
			"items": [
				{"meta": {"node": "node-a", "object": "prod/svc/mysql", "rid": "ip#1"}, "data": {
					"config": {"is_disabled": false, "is_monitored": true, "is_standby": false},
					"monitor": {"restart": {"remaining": 2, "last_at": "2026-07-15T10:00:00Z"}},
					"status": {"type": "ip.host", "label": "192.0.2.10", "status": "down", "monitor": true,
						"provisioned": {"state": "true", "mtime": "2026-07-15T09:00:00Z"},
						"tags": ["front", "critical"], "log": [{"level": "error", "message": "address not available"}]}
				}}
			]
		}`,
	}

	result, err := New(client).ListObjectResources(context.Background(), ListObjectResourcesOptions{
		Path: "prod/svc/mysql", Node: " node-a ", Limit: 100,
	})
	if err != nil {
		t.Fatalf("list object resources: %v", err)
	}
	if result.Total != 1 || result.Count != 1 || result.Truncated {
		t.Fatalf("got unexpected resource list %+v", result)
	}
	resource := result.Resources[0]
	if resource.RID != "ip#1" || resource.Status != "down" || resource.RestartRemaining != 2 {
		t.Errorf("got unexpected resource %+v", resource)
	}
	if len(resource.Logs) != 1 || resource.Logs[0].Message != "address not available" {
		t.Errorf("got unexpected resource logs %+v", resource.Logs)
	}
	if len(resource.Tags) != 2 || resource.Tags[0] != "critical" {
		t.Errorf("got unsorted resource tags %+v", resource.Tags)
	}
}

func TestListObjectResourcesRejectsInvalidCursorBeforeDaemonCall(t *testing.T) {
	client := &recordingJSONGetter{t: t}
	_, err := New(client).ListObjectResources(context.Background(), ListObjectResourcesOptions{Path: "prod/svc/mysql", Cursor: "not-base64!"})
	if err == nil {
		t.Fatal("expected invalid cursor error")
	}
	if client.calls != 0 {
		t.Fatalf("got %d daemon calls, want 0", client.calls)
	}
}
