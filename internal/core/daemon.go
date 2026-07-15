package core

import (
	"context"
	"fmt"
	"net/url"
)

type JSONGetter interface {
	GetJSON(context.Context, string, url.Values, any) error
}

type JSONPoster interface {
	PostJSON(context.Context, string, url.Values, any, any) error
}

type Service struct {
	client JSONGetter
}

func New(client JSONGetter) *Service {
	return &Service{client: client}
}

type DaemonIdentity struct {
	Daemon   DaemonProcessIdentity `json:"daemon" jsonschema:"identity of the local OpenSVC daemon process"`
	Cluster  ClusterIdentity       `json:"cluster" jsonschema:"identity of the OpenSVC cluster"`
	Node     NodeIdentity          `json:"node" jsonschema:"identity and role of the local OpenSVC node"`
	Listener ListenerIdentity      `json:"listener" jsonschema:"OpenSVC daemon listener configuration"`
}

type DaemonProcessIdentity struct {
	NodeName  string `json:"nodename" jsonschema:"the OpenSVC daemon node name"`
	PID       int    `json:"pid" jsonschema:"the OpenSVC daemon process identifier"`
	StartedAt string `json:"started_at" jsonschema:"the OpenSVC daemon start timestamp"`
	Routines  int    `json:"routines" jsonschema:"the number of daemon goroutines"`
}

type ClusterIdentity struct {
	ID     string   `json:"id" jsonschema:"the OpenSVC cluster identifier"`
	Name   string   `json:"name" jsonschema:"the OpenSVC cluster name"`
	Nodes  []string `json:"nodes" jsonschema:"the configured OpenSVC cluster node names"`
	Quorum bool     `json:"quorum" jsonschema:"whether cluster quorum is enabled"`
}

type NodeIdentity struct {
	AgentVersion string `json:"agent_version" jsonschema:"the OpenSVC agent version reported by the local node"`
	APIVersion   int    `json:"api_version" jsonschema:"the OpenSVC daemon API compatibility version"`
	Compat       int    `json:"compat_version" jsonschema:"the OpenSVC daemon compatibility version"`
	IsLeader     bool   `json:"is_leader" jsonschema:"whether the local node is cluster leader"`
	IsOverloaded bool   `json:"is_overloaded" jsonschema:"whether the local node is overloaded"`
	BootedAt     string `json:"booted_at" jsonschema:"the local node boot timestamp"`
}

type ListenerIdentity struct {
	Address string `json:"address" jsonschema:"the configured OpenSVC daemon listener address"`
	Port    int    `json:"port" jsonschema:"the configured OpenSVC daemon listener port"`
}

type clusterStatusResponse struct {
	Cluster struct {
		Config struct {
			ID       string   `json:"id"`
			Name     string   `json:"name"`
			Nodes    []string `json:"nodes"`
			Quorum   bool     `json:"quorum"`
			Listener struct {
				Address string `json:"addr"`
				Port    int    `json:"port"`
			} `json:"listener"`
		} `json:"config"`
		Status struct {
			IsCompatible bool `json:"is_compat"`
			IsFrozen     bool `json:"is_frozen"`
		} `json:"status"`
		Node map[string]struct {
			Status struct {
				Agent        string `json:"agent"`
				API          int    `json:"api"`
				Compat       int    `json:"compat"`
				IsLeader     bool   `json:"is_leader"`
				IsOverloaded bool   `json:"is_overloaded"`
				BootedAt     string `json:"booted_at"`
				FrozenAt     string `json:"frozen_at"`
			} `json:"status"`
			Monitor struct {
				State               string `json:"state"`
				GlobalExpect        string `json:"global_expect"`
				LocalExpect         string `json:"local_expect"`
				OrchestrationID     string `json:"orchestration_id"`
				OrchestrationIsDone bool   `json:"orchestration_is_done"`
				UpdatedAt           string `json:"updated_at"`
			} `json:"monitor"`
			Daemon struct {
				PID       int    `json:"pid"`
				StartedAt string `json:"started_at"`
			} `json:"daemon"`
		} `json:"node"`
		Object map[string]struct {
			Availability     *string  `json:"avail"`
			Overall          string   `json:"overall"`
			Provisioned      string   `json:"provisioned"`
			Frozen           string   `json:"frozen"`
			PlacementState   string   `json:"placement_state"`
			Orchestrate      string   `json:"orchestrate"`
			UpInstancesCount int      `json:"up_instances_count"`
			Scope            []string `json:"scope"`
		} `json:"object"`
	} `json:"cluster"`
	Daemon struct {
		NodeName string `json:"nodename"`
		Routines int    `json:"routines"`
	} `json:"daemon"`
}

func (s *Service) getClusterStatus(ctx context.Context) (clusterStatusResponse, error) {
	var status clusterStatusResponse
	err := s.client.GetJSON(ctx, "/api/cluster/status", url.Values{"selector": {"**"}}, &status)
	if err != nil {
		return clusterStatusResponse{}, fmt.Errorf("get cluster status: %w", err)
	}
	return status, nil
}

func (s *Service) GetDaemonIdentity(ctx context.Context) (DaemonIdentity, error) {
	status, err := s.getClusterStatus(ctx)
	if err != nil {
		return DaemonIdentity{}, fmt.Errorf("get daemon identity: %w", err)
	}

	nodeName := status.Daemon.NodeName
	if nodeName == "" {
		return DaemonIdentity{}, fmt.Errorf("cluster status has no daemon nodename")
	}
	node, ok := status.Cluster.Node[nodeName]
	if !ok {
		return DaemonIdentity{}, fmt.Errorf("cluster status has no data for local node %q", nodeName)
	}
	if node.Status.Agent == "" {
		return DaemonIdentity{}, fmt.Errorf("cluster status has no agent version for local node %q", nodeName)
	}

	return DaemonIdentity{
		Daemon:   DaemonProcessIdentity{NodeName: nodeName, PID: node.Daemon.PID, StartedAt: node.Daemon.StartedAt, Routines: status.Daemon.Routines},
		Cluster:  ClusterIdentity{ID: status.Cluster.Config.ID, Name: status.Cluster.Config.Name, Nodes: status.Cluster.Config.Nodes, Quorum: status.Cluster.Config.Quorum},
		Node:     NodeIdentity{AgentVersion: node.Status.Agent, APIVersion: node.Status.API, Compat: node.Status.Compat, IsLeader: node.Status.IsLeader, IsOverloaded: node.Status.IsOverloaded, BootedAt: node.Status.BootedAt},
		Listener: ListenerIdentity{Address: status.Cluster.Config.Listener.Address, Port: status.Cluster.Config.Listener.Port},
	}, nil
}
