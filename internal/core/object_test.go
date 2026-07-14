package core

import (
	"context"
	"encoding/json"
	"net/url"
	"reflect"
	"testing"
)

type objectJSONGetter struct {
	t        *testing.T
	payload  string
	selector string
	calls    int
}

func (f *objectJSONGetter) GetJSON(_ context.Context, path string, query url.Values, output any) error {
	f.t.Helper()
	f.calls++
	if path != "/api/object/path" {
		f.t.Errorf("got path %q, want /api/object/path", path)
	}
	if got := query.Get("path"); got != f.selector {
		f.t.Errorf("got selector %q, want %q", got, f.selector)
	}
	return json.Unmarshal([]byte(f.payload), output)
}

func TestListClusterObjects(t *testing.T) {
	client := &objectJSONGetter{
		t:        t,
		selector: "lab/**",
		payload: `[
			"system/sec/ca",
			"lab/vol/data",
			"lab/svc/redis",
			"cluster",
			"lab/svc/api",
			"lab/svc/redis"
		]`,
	}
	service := New(client)

	result, err := service.ListClusterObjects(context.Background(), ListClusterObjectsOptions{
		Selector: " lab/** ",
		Limit:    2,
		Cursor:   "lab/svc/a",
	})
	if err != nil {
		t.Fatalf("list cluster objects: %v", err)
	}
	if client.calls != 1 {
		t.Fatalf("got %d daemon calls, want 1", client.calls)
	}
	if result.Selector != "lab/**" || result.Total != 5 || result.Count != 2 {
		t.Errorf("got list metadata %+v, want selector lab/**, total 5, count 2", result)
	}
	if !result.Truncated || result.NextCursor != "lab/svc/redis" {
		t.Errorf("got pagination truncated=%v next_cursor=%q", result.Truncated, result.NextCursor)
	}
	want := []ClusterObjectReference{
		{Path: "lab/svc/api", Namespace: "lab", Kind: "svc", Name: "api"},
		{Path: "lab/svc/redis", Namespace: "lab", Kind: "svc", Name: "redis"},
	}
	if !reflect.DeepEqual(result.Objects, want) {
		t.Errorf("got objects %+v, want %+v", result.Objects, want)
	}
}

func TestListClusterObjectsDefaultsAndEmptyResult(t *testing.T) {
	client := &objectJSONGetter{t: t, selector: "**", payload: `[]`}
	result, err := New(client).ListClusterObjects(context.Background(), ListClusterObjectsOptions{})
	if err != nil {
		t.Fatalf("list cluster objects: %v", err)
	}
	if result.Selector != "**" || result.Total != 0 || result.Count != 0 || result.Truncated {
		t.Fatalf("got unexpected empty result %+v", result)
	}
	if result.Objects == nil {
		t.Fatal("objects must be an empty array, not nil")
	}
}

func TestListClusterObjectsRejectsInvalidLimitBeforeDaemonCall(t *testing.T) {
	client := &objectJSONGetter{t: t, selector: "**", payload: `[]`}
	_, err := New(client).ListClusterObjects(context.Background(), ListClusterObjectsOptions{Limit: 201})
	if err == nil {
		t.Fatal("expected invalid limit error")
	}
	if client.calls != 0 {
		t.Fatalf("got %d daemon calls, want 0", client.calls)
	}
}

func TestParseClusterObjectReference(t *testing.T) {
	tests := []struct {
		path string
		want ClusterObjectReference
	}{
		{path: "cluster", want: ClusterObjectReference{Path: "cluster", Namespace: "root", Kind: "ccfg", Name: "cluster"}},
		{path: "redis", want: ClusterObjectReference{Path: "redis", Namespace: "root", Kind: "svc", Name: "redis"}},
		{path: "cfg/app", want: ClusterObjectReference{Path: "cfg/app", Namespace: "root", Kind: "cfg", Name: "app"}},
		{path: "lab/svc/redis", want: ClusterObjectReference{Path: "lab/svc/redis", Namespace: "lab", Kind: "svc", Name: "redis"}},
		{path: "lab/", want: ClusterObjectReference{Path: "lab/", Namespace: "lab", Kind: "nscfg", Name: "namespace"}},
	}
	for _, test := range tests {
		t.Run(test.path, func(t *testing.T) {
			got, err := parseClusterObjectReference(test.path)
			if err != nil {
				t.Fatalf("parse path: %v", err)
			}
			if got != test.want {
				t.Errorf("got %+v, want %+v", got, test.want)
			}
		})
	}
}

func TestParseClusterObjectReferenceRejectsInvalidPath(t *testing.T) {
	for _, path := range []string{"", "/svc/app", "lab//app", "lab/svc/app/extra"} {
		t.Run(path, func(t *testing.T) {
			if _, err := parseClusterObjectReference(path); err == nil {
				t.Fatalf("expected %q to be rejected", path)
			}
		})
	}
}
