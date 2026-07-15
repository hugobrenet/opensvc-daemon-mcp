package client

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/hugobrenet/opensvc-daemon-mcp/internal/auth"
)

func TestGetJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodGet {
			t.Errorf("got method %q, want GET", request.Method)
		}
		if request.URL.Path != "/api/test" {
			t.Errorf("got path %q, want /api/test", request.URL.Path)
		}
		if got := request.URL.Query().Get("selector"); got != "**" {
			t.Errorf("got selector %q, want **", got)
		}
		if got := request.Header.Get("Accept"); got != "application/json" {
			t.Errorf("got Accept header %q, want application/json", got)
		}
		if got := request.Header.Get("Authorization"); got != "Bearer delegated-token" {
			t.Errorf("got Authorization header %q, want delegated Bearer token", got)
		}
		response.Header().Set("Content-Type", "application/json")
		fmt.Fprint(response, `{"value":"ok"}`)
	}))
	defer server.Close()

	apiClient, err := New(server.URL, server.Client())
	if err != nil {
		t.Fatalf("create API client: %v", err)
	}
	var output struct {
		Value string `json:"value"`
	}
	ctx := auth.WithBearerToken(context.Background(), "delegated-token")
	err = apiClient.GetJSON(ctx, "/api/test", url.Values{"selector": {"**"}}, &output)
	if err != nil {
		t.Fatalf("GET JSON: %v", err)
	}
	if output.Value != "ok" {
		t.Errorf("got value %q, want ok", output.Value)
	}
}

func TestPostJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodPost {
			t.Errorf("got method %q, want POST", request.Method)
		}
		if request.URL.Path != "/api/action/status" {
			t.Errorf("got path %q, want /api/action/status", request.URL.Path)
		}
		if got := request.Header.Get("Authorization"); got != "Bearer delegated-token" {
			t.Errorf("got Authorization header %q, want delegated Bearer token", got)
		}
		var input struct {
			Reason string `json:"reason"`
		}
		if err := json.NewDecoder(request.Body).Decode(&input); err != nil {
			t.Fatalf("decode request body: %v", err)
		}
		if input.Reason != "test" {
			t.Errorf("got reason %q, want test", input.Reason)
		}
		response.Header().Set("Content-Type", "application/json")
		fmt.Fprint(response, `{"session_id":"session-1"}`)
	}))
	defer server.Close()

	apiClient, err := New(server.URL, server.Client())
	if err != nil {
		t.Fatalf("create API client: %v", err)
	}
	var output struct {
		SessionID string `json:"session_id"`
	}
	ctx := auth.WithBearerToken(context.Background(), "delegated-token")
	if err := apiClient.PostJSON(ctx, "/api/action/status", nil, map[string]string{"reason": "test"}, &output); err != nil {
		t.Fatalf("POST JSON: %v", err)
	}
	if output.SessionID != "session-1" {
		t.Errorf("got session id %q, want session-1", output.SessionID)
	}
}

func TestGetJSONHTTPError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, _ *http.Request) {
		http.Error(response, "unauthorized", http.StatusUnauthorized)
	}))
	defer server.Close()

	apiClient, err := New(server.URL, server.Client())
	if err != nil {
		t.Fatalf("create API client: %v", err)
	}
	ctx := auth.WithBearerToken(context.Background(), "delegated-token")
	err = apiClient.GetJSON(ctx, "/api/test", nil, &struct{}{})
	if err == nil {
		t.Fatal("GET JSON succeeded, want an error")
	}
	if !strings.Contains(err.Error(), "401 Unauthorized") {
		t.Fatalf("got error %q, want HTTP 401 status", err)
	}
	var apiError *APIError
	if !errors.As(err, &apiError) {
		t.Fatalf("got error type %T, want *APIError", err)
	}
	if apiError.Method != http.MethodGet || apiError.Path != "/api/test" || apiError.StatusCode != http.StatusUnauthorized {
		t.Errorf("got unexpected API error metadata: %+v", apiError)
	}
	if apiError.Title != "" || apiError.Detail != "" {
		t.Errorf("non-JSON response populated problem fields: %+v", apiError)
	}
}

func TestPostJSONProblemError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodPost {
			t.Errorf("got method %q, want POST", request.Method)
		}
		response.Header().Set("Content-Type", "application/problem+json")
		response.WriteHeader(http.StatusForbidden)
		fmt.Fprint(response, `{"status":418,"title":"Forbidden","detail":"  need one of [operator:lab admin:lab operator admin root]\n grant  "}`)
	}))
	defer server.Close()

	apiClient, err := New(server.URL, server.Client())
	if err != nil {
		t.Fatalf("create API client: %v", err)
	}
	ctx := auth.WithBearerToken(context.Background(), "delegated-token")
	err = apiClient.PostJSON(ctx, "/api/action/status", nil, nil, &struct{}{})
	if err == nil {
		t.Fatal("POST JSON succeeded, want an error")
	}
	var apiError *APIError
	if !errors.As(err, &apiError) {
		t.Fatalf("got error type %T, want *APIError", err)
	}
	if apiError.Method != http.MethodPost || apiError.Path != "/api/action/status" || apiError.StatusCode != http.StatusForbidden || apiError.Status != "403 Forbidden" {
		t.Errorf("got unexpected API error metadata: %+v", apiError)
	}
	if apiError.Title != "Forbidden" || apiError.Detail != "need one of [operator:lab admin:lab operator admin root] grant" {
		t.Errorf("got unexpected problem details: %+v", apiError)
	}
	want := "OpenSVC daemon POST /api/action/status returned HTTP 403 Forbidden: need one of [operator:lab admin:lab operator admin root] grant"
	if err.Error() != want {
		t.Errorf("got error %q, want %q", err, want)
	}
}

func TestGetJSONIgnoresOversizedProblemBody(t *testing.T) {
	const marker = "must-not-be-exposed"
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, _ *http.Request) {
		response.Header().Set("Content-Type", "application/problem+json")
		response.WriteHeader(http.StatusBadGateway)
		fmt.Fprintf(response, `{"title":"Bad Gateway","detail":"%s%s"}`, strings.Repeat("x", maxErrorResponseBodySize), marker)
	}))
	defer server.Close()

	apiClient, err := New(server.URL, server.Client())
	if err != nil {
		t.Fatalf("create API client: %v", err)
	}
	ctx := auth.WithBearerToken(context.Background(), "delegated-token")
	err = apiClient.GetJSON(ctx, "/api/test", nil, &struct{}{})
	if err == nil {
		t.Fatal("GET JSON succeeded, want an error")
	}
	var apiError *APIError
	if !errors.As(err, &apiError) {
		t.Fatalf("got error type %T, want *APIError", err)
	}
	if apiError.Title != "" || apiError.Detail != "" {
		t.Errorf("oversized response populated problem fields: %+v", apiError)
	}
	if strings.Contains(err.Error(), marker) {
		t.Fatalf("error exposes oversized response content: %q", err)
	}
}

func TestNewRejectsInvalidURL(t *testing.T) {
	_, err := New("localhost:1215", nil)
	if err == nil {
		t.Fatal("New succeeded, want an error")
	}
}

func TestGetJSONDoesNotExposeJWTInHTTPError(t *testing.T) {
	const token = "secret-jwt-value"
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		if got := request.Header.Get("Authorization"); got != "Bearer "+token {
			t.Errorf("got Authorization header %q, want Bearer token", got)
		}
		http.Error(response, "unauthorized", http.StatusUnauthorized)
	}))
	defer server.Close()

	apiClient, err := New(server.URL, server.Client())
	if err != nil {
		t.Fatalf("create API client: %v", err)
	}
	ctx := auth.WithBearerToken(context.Background(), token)
	err = apiClient.GetJSON(ctx, "/api/test", nil, &struct{}{})
	if err == nil {
		t.Fatal("GET JSON succeeded, want an error")
	}
	if strings.Contains(err.Error(), token) {
		t.Fatalf("error exposes JWT: %q", err)
	}
}

func TestGetJSONRejectsMissingDelegatedJWT(t *testing.T) {
	apiClient, err := New("https://127.0.0.1:1215", http.DefaultClient)
	if err != nil {
		t.Fatalf("create API client: %v", err)
	}
	err = apiClient.GetJSON(context.Background(), "/api/test", nil, &struct{}{})
	if err == nil {
		t.Fatal("GET JSON succeeded, want an error")
	}
	if !strings.Contains(err.Error(), "delegated OpenSVC access JWT is missing") {
		t.Fatalf("got error %q, want missing delegated JWT", err)
	}
}
