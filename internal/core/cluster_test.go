package core

import (
	"context"
	"testing"
)

func TestGetClusterHealthDegraded(t *testing.T) {
	service := New(&fakeJSONGetter{t: t, payload: `{
		"cluster": {
			"config": {"id": "cluster-123", "name": "prod", "nodes": ["node-a", "node-b"]},
			"status": {"is_compat": true, "is_frozen": false},
			"node": {
				"node-a": {
					"status": {"agent": "v3.0.0", "is_leader": true, "frozen_at": "0001-01-01T00:00:00Z"},
					"monitor": {"state": "idle"}
				},
				"node-b": {
					"status": {"agent": "v3.0.0", "is_overloaded": true, "frozen_at": "2026-07-14T10:00:00Z"},
					"monitor": {"state": "maintenance"}
				}
			},
			"object": {
				"prod/svc/api": {
					"avail": "up", "overall": "up", "provisioned": "true",
					"frozen": "unfrozen", "placement_state": "optimal", "up_instances_count": 2,
					"scope": ["node-a", "node-b"]
				},
				"prod/svc/db": {
					"avail": "down", "overall": "down", "provisioned": "false",
					"frozen": "frozen", "placement_state": "non-optimal", "scope": ["node-b"]
				},
				"system/sec/ca": {"overall": "up"}
			}
		}
	}`})

	health, err := service.GetClusterHealth(context.Background())
	if err != nil {
		t.Fatalf("get cluster health: %v", err)
	}
	if health.Healthy {
		t.Fatal("expected degraded cluster health")
	}
	if len(health.Cluster.LeaderNodes) != 1 || health.Cluster.LeaderNodes[0] != "node-a" {
		t.Fatalf("got leaders %v, want [node-a]", health.Cluster.LeaderNodes)
	}
	if health.NodeSummary.Total != 2 || health.NodeSummary.Healthy != 1 {
		t.Errorf("got node summary %+v, want total=2 healthy=1", health.NodeSummary)
	}
	if health.NodeSummary.Frozen != 1 || health.NodeSummary.Overloaded != 1 || health.NodeSummary.NonIdle != 1 {
		t.Errorf("got node summary %+v, want one frozen, overloaded, and non-idle node", health.NodeSummary)
	}
	if health.ObjectSummary.Total != 2 || health.ObjectSummary.Up != 1 || health.ObjectSummary.Down != 1 || health.ObjectSummary.Problems != 1 {
		t.Errorf("got object summary %+v, want two actor objects and one problem", health.ObjectSummary)
	}
	if len(health.ProblemObjects) != 1 || health.ProblemObjects[0].Path != "prod/svc/db" {
		t.Fatalf("got problem objects %+v, want prod/svc/db", health.ProblemObjects)
	}
}

func TestGetClusterHealthHealthy(t *testing.T) {
	service := New(&fakeJSONGetter{t: t, payload: `{
		"cluster": {
			"config": {"id": "cluster-123", "name": "prod", "nodes": ["node-a"]},
			"status": {"is_compat": true, "is_frozen": false},
			"node": {
				"node-a": {
					"status": {"agent": "v3.0.0", "is_leader": true, "frozen_at": "0001-01-01T00:00:00Z"},
					"monitor": {"state": "idle"}
				}
			},
			"object": {
				"prod/svc/api": {
					"avail": "up", "overall": "up", "provisioned": "true",
					"frozen": "unfrozen", "placement_state": "optimal", "up_instances_count": 1,
					"scope": ["node-a"]
				}
			}
		}
	}`})

	health, err := service.GetClusterHealth(context.Background())
	if err != nil {
		t.Fatalf("get cluster health: %v", err)
	}
	if !health.Healthy {
		t.Fatalf("expected healthy cluster, got %+v", health)
	}
	if len(health.Cluster.Issues) != 0 || len(health.ProblemObjects) != 0 {
		t.Fatalf("expected no issues, got %+v", health)
	}
}
