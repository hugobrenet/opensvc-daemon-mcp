package core

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"testing"
	"time"
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

type refreshInstanceClient struct {
	t        *testing.T
	posted   bool
	advance  bool
	getCalls int
}

func (f *refreshInstanceClient) GetJSON(_ context.Context, path string, query url.Values, output any) error {
	f.t.Helper()
	f.getCalls++
	if path != "/api/instance" {
		return fmt.Errorf("got GET path %q, want /api/instance", path)
	}
	if query.Get("path") != "lab/svc/redis" || query.Get("node") != "node-a" {
		return fmt.Errorf("got unexpected query %#v", query)
	}
	updatedAt := "2026-07-15T10:00:00Z"
	availability := "up"
	if f.posted && f.advance {
		updatedAt = "2026-07-15T10:00:01Z"
		availability = "down"
	}
	payload := fmt.Sprintf(`{"items":[{"meta":{"node":"node-a","object":"lab/svc/redis"},"data":{"monitor":{"state":"idle","local_expect":"started"},"status":{"avail":%q,"overall":%q,"updated_at":%q,"resources":{"container#redis":{"status":%q}}}}}]}`, availability, availability, updatedAt, availability)
	return json.Unmarshal([]byte(payload), output)
}

func (f *refreshInstanceClient) PostJSON(_ context.Context, path string, query url.Values, input any, output any) error {
	f.t.Helper()
	if path != "/api/node/name/node-a/instance/path/lab/svc/redis/action/status" {
		return fmt.Errorf("got POST path %q", path)
	}
	if len(query) != 0 || input != nil {
		return fmt.Errorf("got unexpected POST query or input")
	}
	f.posted = true
	return json.Unmarshal([]byte(`{"session_id":"session-1"}`), output)
}

func TestRefreshInstanceStatus(t *testing.T) {
	client := &refreshInstanceClient{t: t, advance: true}
	result, err := New(client).RefreshInstanceStatus(context.Background(), RefreshInstanceStatusOptions{
		Path: "lab/svc/redis", Node: "node-a", Timeout: 5 * time.Second,
	})
	if err != nil {
		t.Fatalf("refresh instance status: %v", err)
	}
	if !client.posted || client.getCalls != 2 {
		t.Fatalf("got posted=%v getCalls=%d, want true and 2", client.posted, client.getCalls)
	}
	if !result.RefreshObserved || result.TimedOut {
		t.Fatalf("got unexpected refresh flags %+v", result)
	}
	if result.SessionID != "session-1" || result.PreviousUpdatedAt != "2026-07-15T10:00:00Z" || result.CurrentUpdatedAt != "2026-07-15T10:00:01Z" {
		t.Errorf("got unexpected refresh metadata %+v", result)
	}
	if result.Instance.Availability != "down" || result.Instance.ResourceSummary.Down != 1 {
		t.Errorf("got unexpected refreshed instance %+v", result.Instance)
	}
}

func TestRefreshInstanceStatusReturnsStructuredTimeout(t *testing.T) {
	client := &refreshInstanceClient{t: t}
	result, err := New(client).RefreshInstanceStatus(context.Background(), RefreshInstanceStatusOptions{
		Path: "lab/svc/redis", Node: "node-a", Timeout: 5 * time.Second,
	})
	if err != nil {
		t.Fatalf("refresh instance status: %v", err)
	}
	if result.RefreshObserved || !result.TimedOut {
		t.Fatalf("got unexpected refresh flags %+v", result)
	}
	if !client.posted || result.SessionID != "session-1" {
		t.Errorf("timeout lost accepted action metadata: client=%+v result=%+v", client, result)
	}
	if result.CurrentUpdatedAt != result.PreviousUpdatedAt || result.Instance.UpdatedAt != result.PreviousUpdatedAt {
		t.Errorf("timeout did not return the last observed instance: %+v", result)
	}
}

func TestRefreshInstanceStatusRejectsInvalidTimeoutBeforeDaemonCall(t *testing.T) {
	client := &refreshInstanceClient{t: t}
	_, err := New(client).RefreshInstanceStatus(context.Background(), RefreshInstanceStatusOptions{
		Path: "lab/svc/redis", Node: "node-a", Timeout: time.Second,
	})
	if err == nil {
		t.Fatal("expected invalid timeout error")
	}
	if client.getCalls != 0 || client.posted {
		t.Fatalf("daemon was called before validation: %+v", client)
	}
}
