package core

import (
	"context"
	"encoding/json"
	"net/url"
	"reflect"
	"strings"
	"testing"
)

func TestGetObjectConfig(t *testing.T) {
	client := &recordingJSONGetter{
		t:    t,
		path: "/api/object/path/lab/svc/redis/config",
		query: url.Values{
			"evaluate": {"false"},
			"kw":       {"container#redis.image", "container#redis.image_pull_policy"},
		},
		payload: `{
			"kind":"KeywordList",
			"items":[
				{"object":"lab/svc/redis","node":"","keyword":"container#redis.image_pull_policy","value":"once","evaluated_as":""},
				{"object":"lab/svc/redis","node":"","keyword":"container#redis.image","value":"redis:7-alpine","evaluated_as":""}
			]
		}`,
	}

	result, err := New(client).GetObjectConfig(context.Background(), GetObjectConfigOptions{
		Path: " lab/svc/redis ",
		Keywords: []string{
			" container#redis.image_pull_policy ",
			"container#redis.image",
			"container#redis.image",
		},
	})
	if err != nil {
		t.Fatalf("get object config: %v", err)
	}
	if result.Object.Path != "lab/svc/redis" || result.Total != 2 || result.Count != 2 || result.Truncated || result.ValuesTruncated != 0 {
		t.Fatalf("got unexpected object config metadata %+v", result)
	}
	wantFilter := []string{"container#redis.image", "container#redis.image_pull_policy"}
	if !reflect.DeepEqual(result.KeywordFilter, wantFilter) {
		t.Errorf("got keyword filter %#v, want %#v", result.KeywordFilter, wantFilter)
	}
	wantItems := []ObjectConfigKeyword{
		{Keyword: "container#redis.image", Value: "redis:7-alpine"},
		{Keyword: "container#redis.image_pull_policy", Value: "once"},
	}
	if !reflect.DeepEqual(result.Items, wantItems) {
		t.Errorf("got items %#v, want %#v", result.Items, wantItems)
	}
}

func TestGetObjectConfigBoundsItemsAndValues(t *testing.T) {
	longValue := strings.Repeat("é", maxObjectConfigValueRunes+1)
	client := &recordingJSONGetter{
		t:     t,
		path:  "/api/object/path/lab/svc/redis/config",
		query: url.Values{"evaluate": {"false"}},
		payload: `{
			"kind":"KeywordList",
			"items":[
				{"object":"lab/svc/redis","keyword":"z.option","value":"omitted","evaluated_as":""},
				{"object":"lab/svc/redis","keyword":"a.option","value":` + quoteJSON(t, longValue) + `,"evaluated_as":""}
			]
		}`,
	}

	result, err := New(client).GetObjectConfig(context.Background(), GetObjectConfigOptions{
		Path: "lab/svc/redis", Limit: 1,
	})
	if err != nil {
		t.Fatalf("get object config: %v", err)
	}
	if result.Total != 2 || result.Count != 1 || !result.Truncated || result.ValuesTruncated != 1 {
		t.Fatalf("got unexpected bounded metadata %+v", result)
	}
	if got := []rune(result.Items[0].Value); len(got) != maxObjectConfigValueRunes || got[len(got)-1] != '…' || !result.Items[0].ValueTruncated {
		t.Errorf("value was not bounded by runes: length=%d item=%+v", len(got), result.Items[0])
	}
}

func TestGetObjectConfigAppliesAggregateValueBudget(t *testing.T) {
	response := daemonObjectConfig{Kind: "KeywordList"}
	for index := 0; index < 17; index++ {
		response.Items = append(response.Items, daemonObjectConfigItem{
			Object:  "lab/svc/redis",
			Keyword: "option." + string(rune('a'+index)),
			Value:   strings.Repeat("x", maxObjectConfigValueRunes),
		})
	}
	payload, err := json.Marshal(response)
	if err != nil {
		t.Fatalf("marshal daemon response: %v", err)
	}
	client := &recordingJSONGetter{
		t: t, path: "/api/object/path/lab/svc/redis/config",
		query: url.Values{"evaluate": {"false"}}, payload: string(payload),
	}

	result, err := New(client).GetObjectConfig(context.Background(), GetObjectConfigOptions{Path: "lab/svc/redis"})
	if err != nil {
		t.Fatalf("get object config: %v", err)
	}
	if result.Count != 17 || result.Truncated || result.ValuesTruncated != 1 {
		t.Fatalf("got unexpected aggregate budget metadata %+v", result)
	}
	if result.Items[16].Value != "" || !result.Items[16].ValueTruncated {
		t.Errorf("last value was not omitted after exhausting aggregate budget: %+v", result.Items[16])
	}
}

func TestGetObjectConfigRejectsInvalidInputBeforeDaemonCall(t *testing.T) {
	tests := []struct {
		name    string
		options GetObjectConfigOptions
	}{
		{name: "limit", options: GetObjectConfigOptions{Path: "lab/svc/redis", Limit: maxGetObjectConfigLimit + 1}},
		{name: "empty keyword", options: GetObjectConfigOptions{Path: "lab/svc/redis", Keywords: []string{" "}}},
		{name: "long keyword", options: GetObjectConfigOptions{Path: "lab/svc/redis", Keywords: []string{strings.Repeat("x", maxObjectConfigKeywordLength+1)}}},
		{name: "too many keywords", options: GetObjectConfigOptions{Path: "lab/svc/redis", Keywords: make([]string, maxObjectConfigKeywordFilters+1)}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			client := &recordingJSONGetter{t: t}
			if _, err := New(client).GetObjectConfig(context.Background(), test.options); err == nil {
				t.Fatal("GetObjectConfig succeeded, want an error")
			}
			if client.calls != 0 {
				t.Fatalf("got %d daemon calls, want 0", client.calls)
			}
		})
	}
}

func TestGetObjectConfigRejectsUnexpectedDaemonData(t *testing.T) {
	tests := []struct {
		name    string
		payload string
	}{
		{name: "kind", payload: `{"kind":"OtherList","items":[]}`},
		{name: "object", payload: `{"kind":"KeywordList","items":[{"object":"lab/svc/other","keyword":"id","value":"1"}]}`},
		{name: "empty keyword", payload: `{"kind":"KeywordList","items":[{"object":"lab/svc/redis","keyword":"","value":"1"}]}`},
		{name: "evaluated value", payload: `{"kind":"KeywordList","items":[{"object":"lab/svc/redis","keyword":"id","value":"1","evaluated":"1","evaluated_as":"node-a"}]}`},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			client := &recordingJSONGetter{
				t: t, path: "/api/object/path/lab/svc/redis/config",
				query: url.Values{"evaluate": {"false"}}, payload: test.payload,
			}
			if _, err := New(client).GetObjectConfig(context.Background(), GetObjectConfigOptions{Path: "lab/svc/redis"}); err == nil {
				t.Fatal("GetObjectConfig succeeded, want an error")
			}
		})
	}
}

func quoteJSON(t *testing.T, value string) string {
	t.Helper()
	payload, err := json.Marshal(value)
	if err != nil {
		t.Fatalf("marshal JSON string: %v", err)
	}
	return string(payload)
}
