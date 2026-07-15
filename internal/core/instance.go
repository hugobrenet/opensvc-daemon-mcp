package core

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"sort"
	"strings"
	"time"
)

const (
	defaultListObjectInstancesLimit = 50
	maxListObjectInstancesLimit     = 100
	defaultRefreshInstanceTimeout   = 30 * time.Second
	minRefreshInstanceTimeout       = 5 * time.Second
	maxRefreshInstanceTimeout       = 120 * time.Second
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

type RefreshInstanceStatusOptions struct {
	Path    string
	Node    string
	Timeout time.Duration
}

type RefreshInstanceStatusResult struct {
	Object            ClusterObjectReference `json:"object" jsonschema:"the canonical OpenSVC object reference"`
	Node              string                 `json:"node" jsonschema:"the refreshed instance node name"`
	SessionID         string                 `json:"session_id" jsonschema:"the OpenSVC session identifier returned when the status action was accepted"`
	RefreshObserved   bool                   `json:"refresh_observed" jsonschema:"whether a newer instance status was observed before timeout"`
	TimedOut          bool                   `json:"timed_out" jsonschema:"whether polling ended before a newer instance status was observed"`
	PreviousUpdatedAt string                 `json:"previous_updated_at" jsonschema:"the instance status timestamp captured before the refresh action"`
	CurrentUpdatedAt  string                 `json:"current_updated_at" jsonschema:"the latest instance status timestamp observed while polling"`
	DurationMS        int64                  `json:"duration_ms" jsonschema:"elapsed milliseconds from action acceptance to the final observation"`
	Instance          ObjectInstanceStatus   `json:"instance" jsonschema:"the latest instance status observed during the refresh workflow"`
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

func (s *Service) RefreshInstanceStatus(ctx context.Context, options RefreshInstanceStatusOptions) (RefreshInstanceStatusResult, error) {
	reference, err := validateExactObjectPath(options.Path)
	if err != nil {
		return RefreshInstanceStatusResult{}, err
	}
	node := strings.TrimSpace(options.Node)
	if node == "" {
		return RefreshInstanceStatusResult{}, fmt.Errorf("instance node is required")
	}
	if len(node) > 255 {
		return RefreshInstanceStatusResult{}, fmt.Errorf("instance node exceeds 255 characters")
	}
	timeout := options.Timeout
	if timeout == 0 {
		timeout = defaultRefreshInstanceTimeout
	}
	if timeout < minRefreshInstanceTimeout || timeout > maxRefreshInstanceTimeout {
		return RefreshInstanceStatusResult{}, fmt.Errorf("refresh timeout must be between %s and %s", minRefreshInstanceTimeout, maxRefreshInstanceTimeout)
	}

	previous, err := s.getExactObjectInstance(ctx, reference.Path, node)
	if err != nil {
		return RefreshInstanceStatusResult{}, fmt.Errorf("get instance before status refresh: %w", err)
	}
	poster, ok := s.client.(JSONPoster)
	if !ok {
		return RefreshInstanceStatusResult{}, fmt.Errorf("OpenSVC daemon client does not support POST requests")
	}
	var accepted struct {
		SessionID string `json:"session_id"`
	}
	endpoint := fmt.Sprintf(
		"/api/node/name/%s/instance/path/%s/%s/%s/action/status",
		node,
		reference.Namespace,
		reference.Kind,
		reference.Name,
	)
	if err := poster.PostJSON(ctx, endpoint, nil, nil, &accepted); err != nil {
		return RefreshInstanceStatusResult{}, fmt.Errorf("request instance status refresh: %w", err)
	}
	if accepted.SessionID == "" {
		return RefreshInstanceStatusResult{}, fmt.Errorf("instance status refresh response has no session_id")
	}

	started := time.Now()
	result := RefreshInstanceStatusResult{
		Object:            reference,
		Node:              node,
		SessionID:         accepted.SessionID,
		PreviousUpdatedAt: previous.UpdatedAt,
		CurrentUpdatedAt:  previous.UpdatedAt,
		Instance:          previous,
	}
	pollCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	for attempt := 0; ; attempt++ {
		timer := time.NewTimer(refreshPollDelay(attempt))
		select {
		case <-pollCtx.Done():
			timer.Stop()
			if ctx.Err() != nil {
				return RefreshInstanceStatusResult{}, ctx.Err()
			}
			result.TimedOut = true
			result.DurationMS = time.Since(started).Milliseconds()
			return result, nil
		case <-timer.C:
		}

		current, err := s.getExactObjectInstance(pollCtx, reference.Path, node)
		if err != nil {
			if errors.Is(pollCtx.Err(), context.DeadlineExceeded) && ctx.Err() == nil {
				result.TimedOut = true
				result.DurationMS = time.Since(started).Milliseconds()
				return result, nil
			}
			return RefreshInstanceStatusResult{}, fmt.Errorf("poll refreshed instance status: %w", err)
		}
		result.Instance = current
		result.CurrentUpdatedAt = current.UpdatedAt
		if current.UpdatedAt != "" && current.UpdatedAt != previous.UpdatedAt {
			result.RefreshObserved = true
			result.DurationMS = time.Since(started).Milliseconds()
			return result, nil
		}
	}
}

func (s *Service) getExactObjectInstance(ctx context.Context, path string, node string) (ObjectInstanceStatus, error) {
	instances, err := s.ListObjectInstances(ctx, ListObjectInstancesOptions{
		Path:  path,
		Node:  node,
		Limit: maxListObjectInstancesLimit,
	})
	if err != nil {
		return ObjectInstanceStatus{}, err
	}
	for _, instance := range instances.Instances {
		if instance.Node == node {
			return instance, nil
		}
	}
	return ObjectInstanceStatus{}, fmt.Errorf("instance %q on node %q was not found or is not visible to the caller", path, node)
}

func refreshPollDelay(attempt int) time.Duration {
	switch attempt {
	case 0:
		return 250 * time.Millisecond
	case 1:
		return 500 * time.Millisecond
	default:
		return time.Second
	}
}
