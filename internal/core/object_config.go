package core

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"sort"
	"strings"
)

const (
	defaultGetObjectConfigLimit    = 100
	maxGetObjectConfigLimit        = 200
	maxObjectConfigKeywordFilters  = 50
	maxObjectConfigKeywordLength   = 255
	maxObjectConfigValueRunes      = 4096
	maxObjectConfigTotalValueRunes = 64 << 10
)

type GetObjectConfigOptions struct {
	Path     string
	Keywords []string
	Limit    int
}

type ObjectConfig struct {
	Object          ClusterObjectReference `json:"object" jsonschema:"the canonical OpenSVC object reference"`
	KeywordFilter   []string               `json:"keyword_filter,omitempty" jsonschema:"the sorted exact keyword filters sent to the daemon"`
	Total           int                    `json:"total" jsonschema:"the number of matching configuration keywords returned by the daemon"`
	Count           int                    `json:"count" jsonschema:"the number of configuration keyword records returned by this tool call"`
	Items           []ObjectConfigKeyword  `json:"items" jsonschema:"the sorted raw non-evaluated configuration keyword records"`
	Truncated       bool                   `json:"truncated" jsonschema:"whether additional keyword records were omitted by the configured item limit"`
	ValuesTruncated int                    `json:"values_truncated" jsonschema:"the number of returned values shortened by per-value or aggregate output bounds"`
}

type ObjectConfigKeyword struct {
	Keyword        string `json:"keyword" jsonschema:"the OpenSVC configuration keyword in section.option form"`
	Value          string `json:"value" jsonschema:"the bounded raw non-evaluated configuration value"`
	Node           string `json:"node" jsonschema:"the node scope reported by OpenSVC, or an empty string for an unscoped value"`
	ValueTruncated bool   `json:"value_truncated" jsonschema:"whether the raw value was shortened to keep the tool response bounded"`
}

type daemonObjectConfig struct {
	Kind  string                   `json:"kind"`
	Items []daemonObjectConfigItem `json:"items"`
}

type daemonObjectConfigItem struct {
	Object      string          `json:"object"`
	Node        string          `json:"node"`
	Keyword     string          `json:"keyword"`
	Value       string          `json:"value"`
	EvaluatedAs string          `json:"evaluated_as"`
	Evaluated   json.RawMessage `json:"evaluated"`
}

func (s *Service) GetObjectConfig(ctx context.Context, options GetObjectConfigOptions) (ObjectConfig, error) {
	reference, err := validateExactObjectPath(options.Path)
	if err != nil {
		return ObjectConfig{}, err
	}
	keywords, err := normalizeObjectConfigKeywords(options.Keywords)
	if err != nil {
		return ObjectConfig{}, err
	}
	limit := options.Limit
	if limit == 0 {
		limit = defaultGetObjectConfigLimit
	}
	if limit < 1 || limit > maxGetObjectConfigLimit {
		return ObjectConfig{}, fmt.Errorf("object config limit must be between 1 and %d", maxGetObjectConfigLimit)
	}

	query := url.Values{"evaluate": {"false"}}
	if len(keywords) > 0 {
		query["kw"] = keywords
	}
	endpoint := fmt.Sprintf(
		"/api/object/path/%s/%s/%s/config",
		reference.Namespace,
		reference.Kind,
		reference.Name,
	)
	var response daemonObjectConfig
	if err := s.client.GetJSON(ctx, endpoint, query, &response); err != nil {
		return ObjectConfig{}, fmt.Errorf("get object config: %w", err)
	}
	if response.Kind != "KeywordList" {
		return ObjectConfig{}, fmt.Errorf("object config returned unexpected kind %q", response.Kind)
	}
	for _, item := range response.Items {
		if item.Object != reference.Path {
			return ObjectConfig{}, fmt.Errorf("object config returned keyword for unexpected object %q", item.Object)
		}
		if strings.TrimSpace(item.Keyword) == "" {
			return ObjectConfig{}, fmt.Errorf("object config returned an empty keyword")
		}
		if item.EvaluatedAs != "" || (len(item.Evaluated) > 0 && !bytes.Equal(item.Evaluated, []byte("null"))) {
			return ObjectConfig{}, fmt.Errorf("object config unexpectedly returned evaluated data for keyword %q", item.Keyword)
		}
	}
	sort.Slice(response.Items, func(i, j int) bool {
		if response.Items[i].Keyword == response.Items[j].Keyword {
			return response.Items[i].Node < response.Items[j].Node
		}
		return response.Items[i].Keyword < response.Items[j].Keyword
	})

	end := min(limit, len(response.Items))
	items := make([]ObjectConfigKeyword, 0, end)
	remainingValueRunes := maxObjectConfigTotalValueRunes
	valuesTruncated := 0
	for _, item := range response.Items[:end] {
		value, valueTruncated, usedRunes := boundObjectConfigValue(item.Value, remainingValueRunes)
		remainingValueRunes -= usedRunes
		if valueTruncated {
			valuesTruncated++
		}
		items = append(items, ObjectConfigKeyword{
			Keyword:        item.Keyword,
			Value:          value,
			Node:           item.Node,
			ValueTruncated: valueTruncated,
		})
	}

	return ObjectConfig{
		Object:          reference,
		KeywordFilter:   keywords,
		Total:           len(response.Items),
		Count:           len(items),
		Items:           items,
		Truncated:       end < len(response.Items),
		ValuesTruncated: valuesTruncated,
	}, nil
}

func normalizeObjectConfigKeywords(input []string) ([]string, error) {
	if len(input) > maxObjectConfigKeywordFilters {
		return nil, fmt.Errorf("object config accepts at most %d keyword filters", maxObjectConfigKeywordFilters)
	}
	keywords := make([]string, 0, len(input))
	seen := make(map[string]struct{}, len(input))
	for _, inputKeyword := range input {
		keyword := strings.TrimSpace(inputKeyword)
		if keyword == "" {
			return nil, fmt.Errorf("object config keyword filter is empty")
		}
		if len(keyword) > maxObjectConfigKeywordLength {
			return nil, fmt.Errorf("object config keyword filter exceeds %d characters", maxObjectConfigKeywordLength)
		}
		if _, ok := seen[keyword]; ok {
			continue
		}
		seen[keyword] = struct{}{}
		keywords = append(keywords, keyword)
	}
	sort.Strings(keywords)
	return keywords, nil
}

func boundObjectConfigValue(value string, remainingRunes int) (string, bool, int) {
	runes := []rune(value)
	allowed := min(len(runes), maxObjectConfigValueRunes, max(remainingRunes, 0))
	if allowed == len(runes) {
		return value, false, allowed
	}
	if allowed == 0 {
		return "", len(runes) > 0, 0
	}
	return string(runes[:allowed-1]) + "…", true, allowed
}
