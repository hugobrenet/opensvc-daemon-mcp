package core

import (
	"context"
	"fmt"
	"net/url"
)

type JSONGetter interface {
	GetJSON(context.Context, string, url.Values, any) error
}

type Service struct {
	client JSONGetter
}

func New(client JSONGetter) *Service {
	return &Service{client: client}
}

type ServerIdentity struct {
	Daemon   DaemonIdentity   `json:"daemon" jsonschema:"identity of the local OpenSVC daemon process"`
	Cluster  ClusterIdentity  `json:"cluster" jsonschema:"identity of the OpenSVC cluster"`
	Node     NodeIdentity     `json:"node" jsonschema:"identity and role of the local OpenSVC node"`
	Listener ListenerIdentity `json:"listener" jsonschema:"OpenSVC daemon listener configuration"`
}

type DaemonIdentity struct {
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
		Node map[string]struct {
			Status struct {
				Agent        string `json:"agent"`
				API          int    `json:"api"`
				Compat       int    `json:"compat"`
				IsLeader     bool   `json:"is_leader"`
				IsOverloaded bool   `json:"is_overloaded"`
				BootedAt     string `json:"booted_at"`
			} `json:"status"`
			Daemon struct {
				PID       int    `json:"pid"`
				StartedAt string `json:"started_at"`
			} `json:"daemon"`
		} `json:"node"`
	} `json:"cluster"`
	Daemon struct {
		NodeName string `json:"nodename"`
		Routines int    `json:"routines"`
	} `json:"daemon"`
}

func (s *Service) GetServerIdentity(ctx context.Context) (ServerIdentity, error) {
	var status clusterStatusResponse
	err := s.client.GetJSON(ctx, "/api/cluster/status", url.Values{"selector": {"**"}}, &status)
	if err != nil {
		return ServerIdentity{}, fmt.Errorf("get server identity: %w", err)
	}

	nodeName := status.Daemon.NodeName
	if nodeName == "" {
		return ServerIdentity{}, fmt.Errorf("cluster status has no daemon nodename")
	}
	node, ok := status.Cluster.Node[nodeName]
	if !ok {
		return ServerIdentity{}, fmt.Errorf("cluster status has no data for local node %q", nodeName)
	}
	if node.Status.Agent == "" {
		return ServerIdentity{}, fmt.Errorf("cluster status has no agent version for local node %q", nodeName)
	}

	return ServerIdentity{
		Daemon:   DaemonIdentity{NodeName: nodeName, PID: node.Daemon.PID, StartedAt: node.Daemon.StartedAt, Routines: status.Daemon.Routines},
		Cluster:  ClusterIdentity{ID: status.Cluster.Config.ID, Name: status.Cluster.Config.Name, Nodes: status.Cluster.Config.Nodes, Quorum: status.Cluster.Config.Quorum},
		Node:     NodeIdentity{AgentVersion: node.Status.Agent, APIVersion: node.Status.API, Compat: node.Status.Compat, IsLeader: node.Status.IsLeader, IsOverloaded: node.Status.IsOverloaded, BootedAt: node.Status.BootedAt},
		Listener: ListenerIdentity{Address: status.Cluster.Config.Listener.Address, Port: status.Cluster.Config.Listener.Port},
	}, nil
}
