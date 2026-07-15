package client

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"reflect"
	"strings"
	"testing"

	"github.com/hugobrenet/opensvc-daemon-mcp/internal/auth"
)

func TestGetSSE(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodGet || request.URL.Path != "/api/log" {
			t.Errorf("got %s %s, want GET /api/log", request.Method, request.URL.Path)
		}
		if got := request.URL.Query().Get("lines"); got != "2" {
			t.Errorf("got lines %q, want 2", got)
		}
		if got := request.Header.Get("Accept"); got != "text/event-stream" {
			t.Errorf("got Accept %q, want text/event-stream", got)
		}
		if got := request.Header.Get("Authorization"); got != "Bearer delegated-token" {
			t.Errorf("got Authorization %q", got)
		}
		response.Header().Set("Content-Type", "text/event-stream; charset=utf-8")
		fmt.Fprint(response, ": comment\r\nevent: log\r\nid: 7\r\ndata: first\r\ndata: second\r\n\r\ndata: final")
	}))
	defer server.Close()

	apiClient, err := New(server.URL, server.Client())
	if err != nil {
		t.Fatalf("create API client: %v", err)
	}
	type event struct {
		kind string
		id   string
		data string
	}
	var events []event
	ctx := auth.WithBearerToken(context.Background(), "delegated-token")
	err = apiClient.GetSSE(ctx, "/api/log", url.Values{"lines": {"2"}}, func(kind string, id string, data []byte) error {
		events = append(events, event{kind: kind, id: id, data: string(data)})
		return nil
	})
	if err != nil {
		t.Fatalf("GET SSE: %v", err)
	}
	want := []event{{kind: "log", id: "7", data: "first\nsecond"}, {data: "final"}}
	if !reflect.DeepEqual(events, want) {
		t.Errorf("got events %#v, want %#v", events, want)
	}
}

func TestGetSSEHTTPError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, _ *http.Request) {
		response.Header().Set("Content-Type", "application/problem+json")
		response.WriteHeader(http.StatusForbidden)
		fmt.Fprint(response, `{"title":"Forbidden","detail":"need one of [root] grant"}`)
	}))
	defer server.Close()

	apiClient, err := New(server.URL, server.Client())
	if err != nil {
		t.Fatalf("create API client: %v", err)
	}
	ctx := auth.WithBearerToken(context.Background(), "delegated-token")
	err = apiClient.GetSSE(ctx, "/api/log", nil, func(string, string, []byte) error { return nil })
	var apiError *APIError
	if !errors.As(err, &apiError) || apiError.StatusCode != http.StatusForbidden || apiError.Detail != "need one of [root] grant" {
		t.Fatalf("got error %#v, want bounded APIError", err)
	}
}

func TestGetSSERejectsUnexpectedContentType(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, _ *http.Request) {
		response.Header().Set("Content-Type", "application/json")
		fmt.Fprint(response, `{}`)
	}))
	defer server.Close()

	apiClient, _ := New(server.URL, server.Client())
	ctx := auth.WithBearerToken(context.Background(), "delegated-token")
	err := apiClient.GetSSE(ctx, "/api/log", nil, func(string, string, []byte) error { return nil })
	if err == nil || !strings.Contains(err.Error(), "unexpected content type") {
		t.Fatalf("got error %v, want content type error", err)
	}
}

func TestGetSSEPropagatesConsumerError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, _ *http.Request) {
		response.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprint(response, "data: event\n\n")
	}))
	defer server.Close()

	apiClient, _ := New(server.URL, server.Client())
	ctx := auth.WithBearerToken(context.Background(), "delegated-token")
	want := errors.New("reject event")
	err := apiClient.GetSSE(ctx, "/api/log", nil, func(string, string, []byte) error { return want })
	if !errors.Is(err, want) {
		t.Fatalf("got error %v, want consumer error", err)
	}
}

func TestGetSSERejectsOversizedResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, _ *http.Request) {
		response.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprint(response, "data: "+strings.Repeat("x", maxSSEResponseBodySize)+"\n\n")
	}))
	defer server.Close()

	apiClient, _ := New(server.URL, server.Client())
	ctx := auth.WithBearerToken(context.Background(), "delegated-token")
	err := apiClient.GetSSE(ctx, "/api/log", nil, func(string, string, []byte) error { return nil })
	if err == nil {
		t.Fatal("GET SSE succeeded, want oversized response error")
	}
}
