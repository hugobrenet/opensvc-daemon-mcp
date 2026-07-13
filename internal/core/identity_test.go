package core

import (
	"context"
	"encoding/json"
	"net/url"
	"testing"
)

type fakeJSONGetter struct {
	t       *testing.T
	payload string
}

func (f *fakeJSONGetter) GetJSON(_ context.Context, path string, query url.Values, output any) error {
	f.t.Helper()
	if path != "/api/cluster/status" {
		f.t.Errorf("got path %q, want /api/cluster/status", path)
	}
	if got := query.Get("selector"); got != "**" {
		f.t.Errorf("got selector %q, want **", got)
	}
	return json.Unmarshal([]byte(f.payload), output)
}

func TestGetServerIdentity(t *testing.T) {
	service := New(&fakeJSONGetter{t: t, payload: `{
		"cluster": {
			"config": {
				"id": "cluster-123", "name": "prod", "nodes": ["node-a", "node-b"],
				"quorum": true, "listener": {"addr": "::", "port": 1215}
			},
			"node": {
				"node-a": {
					"status": {
						"agent": "v3.0.0", "api": 1, "compat": 2,
						"is_leader": true, "is_overloaded": false,
						"booted_at": "2026-07-10T17:23:14+09:00"
					},
					"daemon": {"pid": 2610, "started_at": "2026-07-10T17:23:35+09:00"}
				}
			}
		},
		"daemon": {"nodename": "node-a", "routines": 121}
	}`})

	identity, err := service.GetServerIdentity(context.Background())
	if err != nil {
		t.Fatalf("get server identity: %v", err)
	}
	if identity.Daemon.NodeName != "node-a" {
		t.Errorf("got nodename %q, want node-a", identity.Daemon.NodeName)
	}
	if identity.Daemon.PID != 2610 {
		t.Errorf("got PID %d, want 2610", identity.Daemon.PID)
	}
	if identity.Cluster.ID != "cluster-123" {
		t.Errorf("got cluster ID %q, want cluster-123", identity.Cluster.ID)
	}
	if identity.Node.AgentVersion != "v3.0.0" {
		t.Errorf("got agent version %q, want v3.0.0", identity.Node.AgentVersion)
	}
	if identity.Listener.Port != 1215 {
		t.Errorf("got listener port %d, want 1215", identity.Listener.Port)
	}
}
