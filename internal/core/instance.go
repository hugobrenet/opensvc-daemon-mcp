package core

import (
	"context"
	"fmt"
	"net/url"
	"sort"
	"strings"
)

const (
	defaultListObjectInstancesLimit = 50
	maxListObjectInstancesLimit     = 100
)

type ListObjectInstancesOptions struct {
	Path   string
	Node   string
	Limit  int
	Cursor string
}

type ObjectInstanceList struct {
	Object     ClusterObjectReference `json:"object" jsonschema:"the canonical OpenSVC object reference"`
	NodeFilter string                 `json:"node_filter,omitempty" jsonschema:"the optional node filter used for the daemon request"`
	Total      int                    `json:"total" jsonschema:"the current number of visible instances matching the filters"`
	Count      int                    `json:"count" jsonschema:"the number of instances returned in this page"`
	Instances  []ObjectInstanceStatus `json:"instances" jsonschema:"the instance status records sorted by node name"`
	NextCursor string                 `json:"next_cursor,omitempty" jsonschema:"the cursor to pass to retrieve the next page"`
	Truncated  bool                   `json:"truncated" jsonschema:"whether more matching instances remain after this page"`
}

type ObjectInstanceStatus struct {
	Node                string                `json:"node" jsonschema:"the instance node name"`
	Availability        string                `json:"availability" jsonschema:"the instance availability reported by OpenSVC"`
	Overall             string                `json:"overall" jsonschema:"the instance overall status reported by OpenSVC"`
	Provisioned         string                `json:"provisioned" jsonschema:"the instance provisioned state reported by OpenSVC"`
	FrozenAt            string                `json:"frozen_at" jsonschema:"the instance frozen timestamp reported by OpenSVC"`
	LastStartedAt       string                `json:"last_started_at" jsonschema:"the last instance start timestamp reported by OpenSVC"`
	UpdatedAt           string                `json:"updated_at" jsonschema:"the last instance status update timestamp reported by OpenSVC; use it to assess freshness"`
	MonitorState        string                `json:"monitor_state" jsonschema:"the instance monitor state"`
	GlobalExpect        string                `json:"global_expect" jsonschema:"the instance monitor global target state"`
	LocalExpect         string                `json:"local_expect" jsonschema:"the instance monitor local target state"`
	OrchestrationID     string                `json:"orchestration_id" jsonschema:"the current orchestration identifier, if any"`
	OrchestrationIsDone bool                  `json:"orchestration_is_done" jsonschema:"whether the instance orchestration reports completion"`
	IsLeader            bool                  `json:"is_leader" jsonschema:"whether this instance is the provisioning leader"`
	IsHALeader          bool                  `json:"is_ha_leader" jsonschema:"whether this instance is an HA leader"`
	ResourceSummary     ResourceStatusSummary `json:"resource_summary" jsonschema:"summary of resource statuses for this instance"`
}

type ResourceStatusSummary struct {
	Total         int `json:"total" jsonschema:"number of resources with reported status"`
	Up            int `json:"up" jsonschema:"number of resources with status up or stdby up"`
	Down          int `json:"down" jsonschema:"number of resources with status down or stdby down"`
	Warn          int `json:"warn" jsonschema:"number of resources with status warn"`
	NotApplicable int `json:"not_applicable" jsonschema:"number of resources with status n/a"`
	Other         int `json:"other" jsonschema:"number of resources with any other status"`
}

type daemonInstanceList struct {
	Items []struct {
		Meta struct {
			Node   string `json:"node"`
			Object string `json:"object"`
		} `json:"meta"`
		Data daemonInstance `json:"data"`
	} `json:"items"`
}

type daemonInstance struct {
	Monitor *struct {
		State               string `json:"state"`
		GlobalExpect        string `json:"global_expect"`
		LocalExpect         string `json:"local_expect"`
		OrchestrationID     string `json:"orchestration_id"`
		OrchestrationIsDone bool   `json:"orchestration_is_done"`
		IsLeader            bool   `json:"is_leader"`
		IsHALeader          bool   `json:"is_ha_leader"`
	} `json:"monitor"`
	Status *struct {
		Availability  string                              `json:"avail"`
		Overall       string                              `json:"overall"`
		Provisioned   string                              `json:"provisioned"`
		FrozenAt      string                              `json:"frozen_at"`
		LastStartedAt string                              `json:"last_started_at"`
		UpdatedAt     string                              `json:"updated_at"`
		Resources     map[string]daemonResourceStatusData `json:"resources"`
	} `json:"status"`
}

type daemonResourceStatusData struct {
	Status string `json:"status"`
}

func (s *Service) ListObjectInstances(ctx context.Context, options ListObjectInstancesOptions) (ObjectInstanceList, error) {
	reference, err := validateExactObjectPath(options.Path)
	if err != nil {
		return ObjectInstanceList{}, err
	}
	if len(options.Node) > 255 {
		return ObjectInstanceList{}, fmt.Errorf("node filter exceeds 255 characters")
	}
	if len(options.Cursor) > 255 {
		return ObjectInstanceList{}, fmt.Errorf("instance cursor exceeds 255 characters")
	}
	limit := options.Limit
	if limit == 0 {
		limit = defaultListObjectInstancesLimit
	}
	if limit < 1 || limit > maxListObjectInstancesLimit {
		return ObjectInstanceList{}, fmt.Errorf("instance list limit must be between 1 and %d", maxListObjectInstancesLimit)
	}

	query := url.Values{"path": {reference.Path}}
	if node := strings.TrimSpace(options.Node); node != "" {
		query.Set("node", node)
	}
	var response daemonInstanceList
	if err := s.client.GetJSON(ctx, "/api/instance", query, &response); err != nil {
		return ObjectInstanceList{}, fmt.Errorf("list object instances: %w", err)
	}

	instances := make([]ObjectInstanceStatus, 0, len(response.Items))
	for _, item := range response.Items {
		if item.Meta.Object != reference.Path {
			continue
		}
		instance := ObjectInstanceStatus{Node: item.Meta.Node}
		if item.Data.Status != nil {
			instance.Availability = item.Data.Status.Availability
			instance.Overall = item.Data.Status.Overall
			instance.Provisioned = item.Data.Status.Provisioned
			instance.FrozenAt = item.Data.Status.FrozenAt
			instance.LastStartedAt = item.Data.Status.LastStartedAt
			instance.UpdatedAt = item.Data.Status.UpdatedAt
			instance.ResourceSummary = summarizeResourceStatuses(item.Data.Status.Resources)
		}
		if item.Data.Monitor != nil {
			instance.MonitorState = item.Data.Monitor.State
			instance.GlobalExpect = item.Data.Monitor.GlobalExpect
			instance.LocalExpect = item.Data.Monitor.LocalExpect
			instance.OrchestrationID = item.Data.Monitor.OrchestrationID
			instance.OrchestrationIsDone = item.Data.Monitor.OrchestrationIsDone
			instance.IsLeader = item.Data.Monitor.IsLeader
			instance.IsHALeader = item.Data.Monitor.IsHALeader
		}
		instances = append(instances, instance)
	}
	sort.Slice(instances, func(i, j int) bool { return instances[i].Node < instances[j].Node })

	start := sort.Search(len(instances), func(i int) bool { return instances[i].Node > options.Cursor })
	end := min(start+limit, len(instances))
	page := append([]ObjectInstanceStatus{}, instances[start:end]...)
	result := ObjectInstanceList{
		Object:     reference,
		NodeFilter: strings.TrimSpace(options.Node),
		Total:      len(instances),
		Count:      len(page),
		Instances:  page,
		Truncated:  end < len(instances),
	}
	if result.Truncated {
		result.NextCursor = page[len(page)-1].Node
	}
	return result, nil
}

func summarizeResourceStatuses(resources map[string]daemonResourceStatusData) ResourceStatusSummary {
	var summary ResourceStatusSummary
	for _, resource := range resources {
		summary.Total++
		switch normalizedState(resource.Status) {
		case "up", "stdby up":
			summary.Up++
		case "down", "stdby down":
			summary.Down++
		case "warn":
			summary.Warn++
		case "n/a":
			summary.NotApplicable++
		default:
			summary.Other++
		}
	}
	return summary
}
