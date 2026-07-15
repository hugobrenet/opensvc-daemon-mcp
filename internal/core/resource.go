package core

import (
	"context"
	"encoding/base64"
	"fmt"
	"net/url"
	"sort"
	"strings"
)

const (
	defaultListObjectResourcesLimit = 100
	maxListObjectResourcesLimit     = 200
	maxResourceLogEntries           = 20
)

type ListObjectResourcesOptions struct {
	Path   string
	Node   string
	RID    string
	Limit  int
	Cursor string
}

type ObjectResourceList struct {
	Object     ClusterObjectReference `json:"object" jsonschema:"the canonical OpenSVC object reference"`
	NodeFilter string                 `json:"node_filter,omitempty" jsonschema:"the optional node filter used for the daemon request"`
	RIDFilter  string                 `json:"rid_filter,omitempty" jsonschema:"the optional resource id filter used for the daemon request"`
	Total      int                    `json:"total" jsonschema:"the current number of visible resources matching the filters"`
	Count      int                    `json:"count" jsonschema:"the number of resources returned in this page"`
	Resources  []ObjectResourceStatus `json:"resources" jsonschema:"the resource status records sorted by node, encapsulated node, and resource id"`
	NextCursor string                 `json:"next_cursor,omitempty" jsonschema:"the opaque cursor to pass to retrieve the next page"`
	Truncated  bool                   `json:"truncated" jsonschema:"whether more matching resources remain after this page"`
}

type ObjectResourceStatus struct {
	Node             string             `json:"node" jsonschema:"the instance node name"`
	EncapNode        string             `json:"encap_node,omitempty" jsonschema:"the encapsulated node name when the resource status comes from an encapsulated instance"`
	RID              string             `json:"rid" jsonschema:"the OpenSVC resource identifier"`
	Type             string             `json:"type" jsonschema:"the OpenSVC resource driver type"`
	Label            string             `json:"label" jsonschema:"the resource label reported by OpenSVC"`
	Status           string             `json:"status" jsonschema:"the resource availability status reported by OpenSVC"`
	Provisioned      string             `json:"provisioned" jsonschema:"the resource provisioned state reported by OpenSVC"`
	ProvisionedAt    string             `json:"provisioned_at,omitempty" jsonschema:"the timestamp associated with the resource provisioned state"`
	IsDisabled       bool               `json:"is_disabled" jsonschema:"whether the resource is disabled in instance configuration or status"`
	IsMonitored      bool               `json:"is_monitored" jsonschema:"whether the resource is monitored"`
	IsOptional       bool               `json:"is_optional" jsonschema:"whether the resource is excluded from instance availability aggregation"`
	IsStandby        bool               `json:"is_standby" jsonschema:"whether the resource is configured as standby"`
	IsEncap          bool               `json:"is_encap" jsonschema:"whether the resource is handled by an encapsulated agent"`
	Subset           string             `json:"subset,omitempty" jsonschema:"the resource subset name"`
	Tags             []string           `json:"tags" jsonschema:"the resource tags reported by OpenSVC"`
	RestartRemaining int                `json:"restart_remaining" jsonschema:"the remaining automatic restart attempts"`
	RestartLastAt    string             `json:"restart_last_at,omitempty" jsonschema:"the timestamp of the last automatic restart attempt"`
	Logs             []ResourceLogEntry `json:"logs" jsonschema:"the bounded resource status messages reported by OpenSVC"`
	LogsTruncated    bool               `json:"logs_truncated" jsonschema:"whether additional resource status messages were omitted"`
}

type ResourceLogEntry struct {
	Level   string `json:"level" jsonschema:"the resource status message level"`
	Message string `json:"message" jsonschema:"the resource status message"`
}

type daemonResourceList struct {
	Items []struct {
		Meta struct {
			Node      string `json:"node"`
			Object    string `json:"object"`
			RID       string `json:"rid"`
			EncapNode string `json:"encap_node"`
		} `json:"meta"`
		Data struct {
			Config *struct {
				IsDisabled  bool `json:"is_disabled"`
				IsMonitored bool `json:"is_monitored"`
				IsStandby   bool `json:"is_standby"`
			} `json:"config"`
			Monitor *struct {
				Restart *struct {
					Remaining int    `json:"remaining"`
					LastAt    string `json:"last_at"`
				} `json:"restart"`
			} `json:"monitor"`
			Status *struct {
				Type        string   `json:"type"`
				Label       string   `json:"label"`
				Status      string   `json:"status"`
				Disable     bool     `json:"disable"`
				Monitor     bool     `json:"monitor"`
				Optional    bool     `json:"optional"`
				Standby     bool     `json:"standby"`
				Encap       bool     `json:"encap"`
				Subset      string   `json:"subset"`
				Tags        []string `json:"tags"`
				Provisioned struct {
					State string `json:"state"`
					Mtime string `json:"mtime"`
				} `json:"provisioned"`
				Log []ResourceLogEntry `json:"log"`
			} `json:"status"`
		} `json:"data"`
	} `json:"items"`
}

func (s *Service) ListObjectResources(ctx context.Context, options ListObjectResourcesOptions) (ObjectResourceList, error) {
	reference, err := validateExactObjectPath(options.Path)
	if err != nil {
		return ObjectResourceList{}, err
	}
	if len(options.Node) > 255 {
		return ObjectResourceList{}, fmt.Errorf("node filter exceeds 255 characters")
	}
	if len(options.RID) > 255 {
		return ObjectResourceList{}, fmt.Errorf("resource id filter exceeds 255 characters")
	}
	limit := options.Limit
	if limit == 0 {
		limit = defaultListObjectResourcesLimit
	}
	if limit < 1 || limit > maxListObjectResourcesLimit {
		return ObjectResourceList{}, fmt.Errorf("resource list limit must be between 1 and %d", maxListObjectResourcesLimit)
	}
	cursorKey, err := decodeResourceCursor(options.Cursor)
	if err != nil {
		return ObjectResourceList{}, err
	}

	query := url.Values{"path": {reference.Path}}
	if node := strings.TrimSpace(options.Node); node != "" {
		query.Set("node", node)
	}
	if rid := strings.TrimSpace(options.RID); rid != "" {
		query.Set("resource", rid)
	}
	var response daemonResourceList
	if err := s.client.GetJSON(ctx, "/api/resource", query, &response); err != nil {
		return ObjectResourceList{}, fmt.Errorf("list object resources: %w", err)
	}

	resources := make([]ObjectResourceStatus, 0, len(response.Items))
	for _, item := range response.Items {
		if item.Meta.Object != reference.Path {
			continue
		}
		resource := ObjectResourceStatus{
			Node:      item.Meta.Node,
			EncapNode: item.Meta.EncapNode,
			RID:       item.Meta.RID,
			Tags:      []string{},
			Logs:      []ResourceLogEntry{},
		}
		if item.Data.Config != nil {
			resource.IsDisabled = item.Data.Config.IsDisabled
			resource.IsMonitored = item.Data.Config.IsMonitored
			resource.IsStandby = item.Data.Config.IsStandby
		}
		if item.Data.Monitor != nil && item.Data.Monitor.Restart != nil {
			resource.RestartRemaining = item.Data.Monitor.Restart.Remaining
			resource.RestartLastAt = item.Data.Monitor.Restart.LastAt
		}
		if item.Data.Status != nil {
			resource.Type = item.Data.Status.Type
			resource.Label = item.Data.Status.Label
			resource.Status = item.Data.Status.Status
			resource.Provisioned = item.Data.Status.Provisioned.State
			resource.ProvisionedAt = item.Data.Status.Provisioned.Mtime
			resource.IsDisabled = resource.IsDisabled || item.Data.Status.Disable
			resource.IsMonitored = item.Data.Status.Monitor
			resource.IsOptional = item.Data.Status.Optional
			resource.IsStandby = resource.IsStandby || item.Data.Status.Standby
			resource.IsEncap = item.Data.Status.Encap
			resource.Subset = item.Data.Status.Subset
			resource.Tags = append([]string{}, item.Data.Status.Tags...)
			logs := item.Data.Status.Log
			if len(logs) > maxResourceLogEntries {
				resource.LogsTruncated = true
				logs = logs[:maxResourceLogEntries]
			}
			resource.Logs = append([]ResourceLogEntry{}, logs...)
		}
		sort.Strings(resource.Tags)
		resources = append(resources, resource)
	}
	sort.Slice(resources, func(i, j int) bool { return resourceSortKey(resources[i]) < resourceSortKey(resources[j]) })
	start := sort.Search(len(resources), func(i int) bool { return resourceSortKey(resources[i]) > cursorKey })
	end := min(start+limit, len(resources))
	page := append([]ObjectResourceStatus{}, resources[start:end]...)
	result := ObjectResourceList{
		Object:     reference,
		NodeFilter: strings.TrimSpace(options.Node),
		RIDFilter:  strings.TrimSpace(options.RID),
		Total:      len(resources),
		Count:      len(page),
		Resources:  page,
		Truncated:  end < len(resources),
	}
	if result.Truncated {
		result.NextCursor = base64.RawURLEncoding.EncodeToString([]byte(resourceSortKey(page[len(page)-1])))
	}
	return result, nil
}

func resourceSortKey(resource ObjectResourceStatus) string {
	return resource.Node + "\x00" + resource.EncapNode + "\x00" + resource.RID
}

func decodeResourceCursor(cursor string) (string, error) {
	if cursor == "" {
		return "", nil
	}
	if len(cursor) > 1024 {
		return "", fmt.Errorf("resource cursor exceeds 1024 characters")
	}
	value, err := base64.RawURLEncoding.DecodeString(cursor)
	if err != nil {
		return "", fmt.Errorf("invalid resource cursor")
	}
	return string(value), nil
}
