package core

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"
)

const maxClusterHealthProblemObjects = 100

type ClusterHealth struct {
	Healthy                 bool                       `json:"healthy" jsonschema:"whether all evaluated cluster health checks pass"`
	Cluster                 ClusterHealthStatus        `json:"cluster" jsonschema:"cluster-wide health checks"`
	NodeSummary             ClusterNodeHealthSummary   `json:"node_summary" jsonschema:"summary of evaluated node health"`
	Nodes                   []ClusterNodeHealth        `json:"nodes" jsonschema:"health details for configured and reported nodes"`
	ObjectSummary           ClusterObjectHealthSummary `json:"object_summary" jsonschema:"summary of actor object availability"`
	ProblemObjects          []ClusterObjectHealth      `json:"problem_objects" jsonschema:"actor objects with health issues, sorted by path and limited to 100 entries"`
	ProblemObjectsTruncated bool                       `json:"problem_objects_truncated" jsonschema:"whether more than 100 problem objects were found"`
}

type ClusterHealthStatus struct {
	ID           string   `json:"id" jsonschema:"the OpenSVC cluster identifier"`
	Name         string   `json:"name" jsonschema:"the OpenSVC cluster name"`
	IsCompatible bool     `json:"is_compatible" jsonschema:"whether cluster nodes are mutually compatible"`
	IsFrozen     bool     `json:"is_frozen" jsonschema:"whether the cluster is frozen"`
	LeaderNodes  []string `json:"leader_nodes" jsonschema:"reported cluster leader node names"`
	Issues       []string `json:"issues" jsonschema:"cluster-wide health issues"`
}

type ClusterNodeHealthSummary struct {
	Total      int `json:"total"`
	Healthy    int `json:"healthy"`
	Missing    int `json:"missing"`
	Frozen     int `json:"frozen"`
	Overloaded int `json:"overloaded"`
	NonIdle    int `json:"non_idle"`
}

type ClusterNodeHealth struct {
	Name         string   `json:"name"`
	Reported     bool     `json:"reported"`
	Healthy      bool     `json:"healthy"`
	MonitorState string   `json:"monitor_state"`
	IsLeader     bool     `json:"is_leader"`
	IsFrozen     bool     `json:"is_frozen"`
	IsOverloaded bool     `json:"is_overloaded"`
	Issues       []string `json:"issues"`
}

type ClusterObjectHealthSummary struct {
	Total         int `json:"total"`
	Up            int `json:"up"`
	Down          int `json:"down"`
	Warn          int `json:"warn"`
	NotApplicable int `json:"not_applicable"`
	Other         int `json:"other"`
	Problems      int `json:"problems"`
}

type ClusterObjectHealth struct {
	Path             string   `json:"path"`
	Availability     string   `json:"availability"`
	Overall          string   `json:"overall"`
	Provisioned      string   `json:"provisioned"`
	Frozen           string   `json:"frozen"`
	PlacementState   string   `json:"placement_state"`
	UpInstancesCount int      `json:"up_instances_count"`
	Scope            []string `json:"scope"`
	Issues           []string `json:"issues"`
}

func (s *Service) GetClusterHealth(ctx context.Context) (ClusterHealth, error) {
	status, err := s.getClusterStatus(ctx)
	if err != nil {
		return ClusterHealth{}, fmt.Errorf("get cluster health: %w", err)
	}
	return clusterHealthFromStatus(status), nil
}

func clusterHealthFromStatus(status clusterStatusResponse) ClusterHealth {
	health := ClusterHealth{
		Cluster: ClusterHealthStatus{
			ID:           status.Cluster.Config.ID,
			Name:         status.Cluster.Config.Name,
			IsCompatible: status.Cluster.Status.IsCompatible,
			IsFrozen:     status.Cluster.Status.IsFrozen,
			LeaderNodes:  []string{},
			Issues:       []string{},
		},
		Nodes:          []ClusterNodeHealth{},
		ProblemObjects: []ClusterObjectHealth{},
	}

	if !health.Cluster.IsCompatible {
		health.Cluster.Issues = append(health.Cluster.Issues, "cluster nodes are not compatible")
	}
	if health.Cluster.IsFrozen {
		health.Cluster.Issues = append(health.Cluster.Issues, "cluster is frozen")
	}

	nodeNames := make(map[string]struct{}, len(status.Cluster.Config.Nodes)+len(status.Cluster.Node))
	for _, name := range status.Cluster.Config.Nodes {
		nodeNames[name] = struct{}{}
	}
	for name, node := range status.Cluster.Node {
		nodeNames[name] = struct{}{}
		if node.Status.IsLeader {
			health.Cluster.LeaderNodes = append(health.Cluster.LeaderNodes, name)
		}
	}
	sort.Strings(health.Cluster.LeaderNodes)
	if len(health.Cluster.LeaderNodes) == 0 {
		health.Cluster.Issues = append(health.Cluster.Issues, "cluster has no reported leader")
	} else if len(health.Cluster.LeaderNodes) > 1 {
		health.Cluster.Issues = append(health.Cluster.Issues, "cluster has multiple reported leaders")
	}

	sortedNodeNames := sortedKeys(nodeNames)
	health.NodeSummary.Total = len(sortedNodeNames)
	for _, name := range sortedNodeNames {
		node, reported := status.Cluster.Node[name]
		nodeHealth := ClusterNodeHealth{Name: name, Reported: reported, Issues: []string{}}
		if !reported {
			nodeHealth.Issues = append(nodeHealth.Issues, "configured node has no status data")
			health.NodeSummary.Missing++
		} else {
			nodeHealth.MonitorState = node.Monitor.State
			nodeHealth.IsLeader = node.Status.IsLeader
			nodeHealth.IsFrozen = isNonZeroTimestamp(node.Status.FrozenAt)
			nodeHealth.IsOverloaded = node.Status.IsOverloaded
			if node.Status.Agent == "" {
				nodeHealth.Issues = append(nodeHealth.Issues, "node has no reported agent version")
			}
			if strings.TrimSpace(node.Monitor.State) == "" {
				nodeHealth.Issues = append(nodeHealth.Issues, "node has no monitor state")
			} else if normalizedState(node.Monitor.State) != "idle" {
				nodeHealth.Issues = append(nodeHealth.Issues, fmt.Sprintf("node monitor state is %q", node.Monitor.State))
				health.NodeSummary.NonIdle++
			}
			if nodeHealth.IsFrozen {
				nodeHealth.Issues = append(nodeHealth.Issues, "node is frozen")
				health.NodeSummary.Frozen++
			}
			if nodeHealth.IsOverloaded {
				nodeHealth.Issues = append(nodeHealth.Issues, "node is overloaded")
				health.NodeSummary.Overloaded++
			}
		}
		nodeHealth.Healthy = len(nodeHealth.Issues) == 0
		if nodeHealth.Healthy {
			health.NodeSummary.Healthy++
		}
		health.Nodes = append(health.Nodes, nodeHealth)
	}

	objectPaths := make([]string, 0, len(status.Cluster.Object))
	for path, object := range status.Cluster.Object {
		if object.Availability != nil {
			objectPaths = append(objectPaths, path)
		}
	}
	sort.Strings(objectPaths)
	for _, path := range objectPaths {
		object := status.Cluster.Object[path]
		availability := normalizedState(*object.Availability)
		health.ObjectSummary.Total++
		switch availability {
		case "up", "stdby up":
			health.ObjectSummary.Up++
		case "down", "stdby down":
			health.ObjectSummary.Down++
		case "warn":
			health.ObjectSummary.Warn++
		case "n/a":
			health.ObjectSummary.NotApplicable++
		default:
			health.ObjectSummary.Other++
		}

		objectHealth := ClusterObjectHealth{
			Path:             path,
			Availability:     *object.Availability,
			Overall:          object.Overall,
			Provisioned:      object.Provisioned,
			Frozen:           object.Frozen,
			PlacementState:   object.PlacementState,
			UpInstancesCount: object.UpInstancesCount,
			Scope:            append([]string(nil), object.Scope...),
			Issues:           []string{},
		}
		if availability != "up" && availability != "stdby up" && availability != "n/a" {
			objectHealth.Issues = append(objectHealth.Issues, fmt.Sprintf("availability is %q", displayState(*object.Availability)))
		}
		if isProblemState(object.Overall, "down", "warn", "undef", "stdby down") {
			objectHealth.Issues = append(objectHealth.Issues, fmt.Sprintf("overall status is %q", displayState(object.Overall)))
		}
		if state := normalizedState(object.PlacementState); state != "" && state != "optimal" && state != "n/a" {
			objectHealth.Issues = append(objectHealth.Issues, fmt.Sprintf("placement state is %q", object.PlacementState))
		}
		if state := normalizedState(object.Frozen); state != "" && state != "unfrozen" {
			objectHealth.Issues = append(objectHealth.Issues, fmt.Sprintf("freeze state is %q", object.Frozen))
		}
		if isProblemState(object.Provisioned, "false", "mixed", "undef") {
			objectHealth.Issues = append(objectHealth.Issues, fmt.Sprintf("provisioned state is %q", object.Provisioned))
		}
		if len(objectHealth.Issues) > 0 {
			health.ObjectSummary.Problems++
			if len(health.ProblemObjects) < maxClusterHealthProblemObjects {
				health.ProblemObjects = append(health.ProblemObjects, objectHealth)
			} else {
				health.ProblemObjectsTruncated = true
			}
		}
	}

	health.Healthy = len(health.Cluster.Issues) == 0 &&
		health.NodeSummary.Healthy == health.NodeSummary.Total &&
		health.ObjectSummary.Problems == 0
	return health
}

func sortedKeys(values map[string]struct{}) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func normalizedState(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}

func displayState(value string) string {
	if strings.TrimSpace(value) == "" {
		return "unknown"
	}
	return value
}

func isProblemState(value string, problems ...string) bool {
	state := normalizedState(value)
	for _, problem := range problems {
		if state == problem {
			return true
		}
	}
	return false
}

func isNonZeroTimestamp(value string) bool {
	value = strings.TrimSpace(value)
	if value == "" {
		return false
	}
	timestamp, err := time.Parse(time.RFC3339Nano, value)
	if err != nil {
		return true
	}
	return !timestamp.IsZero()
}
