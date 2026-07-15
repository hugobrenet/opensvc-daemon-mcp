package core

import (
	"context"
	"net/url"
	"testing"
)

func TestListObjectInstances(t *testing.T) {
	client := &recordingJSONGetter{
		t:     t,
		path:  "/api/instance",
		query: url.Values{"path": {"prod/svc/mysql"}},
		payload: `{
			"kind": "InstanceList",
			"items": [
				{"meta": {"node": "node-b", "object": "prod/svc/mysql"}, "data": {
					"monitor": {"state": "idle", "global_expect": "started", "is_ha_leader": false},
					"status": {"avail": "down", "overall": "warn", "provisioned": "true", "resources": {"ip#1": {"status": "down"}}}
				}},
				{"meta": {"node": "node-a", "object": "prod/svc/mysql"}, "data": {
					"monitor": {"state": "idle", "global_expect": "started", "is_ha_leader": true},
					"status": {"avail": "up", "overall": "up", "provisioned": "true", "resources": {"ip#1": {"status": "up"}, "app#1": {"status": "n/a"}}}
				}}
			]
		}`,
	}

	result, err := New(client).ListObjectInstances(context.Background(), ListObjectInstancesOptions{Path: "prod/svc/mysql", Limit: 1})
	if err != nil {
		t.Fatalf("list object instances: %v", err)
	}
	if result.Total != 2 || result.Count != 1 || !result.Truncated || result.NextCursor != "node-a" {
		t.Fatalf("got unexpected page metadata %+v", result)
	}
	instance := result.Instances[0]
	if instance.Node != "node-a" || instance.Availability != "up" || !instance.IsHALeader {
		t.Errorf("got unexpected instance %+v", instance)
	}
	if instance.ResourceSummary.Total != 2 || instance.ResourceSummary.Up != 1 || instance.ResourceSummary.NotApplicable != 1 {
		t.Errorf("got unexpected resource summary %+v", instance.ResourceSummary)
	}
}

func TestListObjectInstancesRejectsInvalidLimitBeforeDaemonCall(t *testing.T) {
	client := &recordingJSONGetter{t: t}
	_, err := New(client).ListObjectInstances(context.Background(), ListObjectInstancesOptions{Path: "prod/svc/mysql", Limit: 101})
	if err == nil {
		t.Fatal("expected invalid limit error")
	}
	if client.calls != 0 {
		t.Fatalf("got %d daemon calls, want 0", client.calls)
	}
}
