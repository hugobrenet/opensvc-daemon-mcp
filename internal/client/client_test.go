package client

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
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
		if got := request.Header.Get("Authorization"); got != "" {
			t.Errorf("got unexpected Authorization header %q", got)
		}
		response.Header().Set("Content-Type", "application/json")
		fmt.Fprint(response, `{"value":"ok"}`)
	}))
	defer server.Close()

	apiClient, err := New(server.URL, server.Client(), auth.None{})
	if err != nil {
		t.Fatalf("create API client: %v", err)
	}
	var output struct {
		Value string `json:"value"`
	}
	err = apiClient.GetJSON(context.Background(), "/api/test", url.Values{"selector": {"**"}}, &output)
	if err != nil {
		t.Fatalf("GET JSON: %v", err)
	}
	if output.Value != "ok" {
		t.Errorf("got value %q, want ok", output.Value)
	}
}

func TestGetJSONHTTPError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, _ *http.Request) {
		http.Error(response, "unauthorized", http.StatusUnauthorized)
	}))
	defer server.Close()

	apiClient, err := New(server.URL, server.Client(), auth.None{})
	if err != nil {
		t.Fatalf("create API client: %v", err)
	}
	err = apiClient.GetJSON(context.Background(), "/api/test", nil, &struct{}{})
	if err == nil {
		t.Fatal("GET JSON succeeded, want an error")
	}
	if !strings.Contains(err.Error(), "401 Unauthorized") {
		t.Fatalf("got error %q, want HTTP 401 status", err)
	}
}

func TestNewRejectsInvalidURL(t *testing.T) {
	_, err := New("localhost:1215", nil, auth.None{})
	if err == nil {
		t.Fatal("New succeeded, want an error")
	}
}

func TestNewRejectsMissingAuthenticator(t *testing.T) {
	_, err := New("https://127.0.0.1:1215", nil, nil)
	if err == nil {
		t.Fatal("New succeeded, want an error")
	}
}

func TestGetJSONDoesNotExposeJWTInHTTPError(t *testing.T) {
	const token = "secret-jwt-value"
	tokenFile := filepath.Join(t.TempDir(), "daemon.jwt")
	if err := os.WriteFile(tokenFile, []byte(token), 0o600); err != nil {
		t.Fatalf("write token file: %v", err)
	}
	authenticator, err := auth.NewJWT(tokenFile)
	if err != nil {
		t.Fatalf("create JWT authenticator: %v", err)
	}

	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		if got := request.Header.Get("Authorization"); got != "Bearer "+token {
			t.Errorf("got Authorization header %q, want Bearer token", got)
		}
		http.Error(response, "unauthorized", http.StatusUnauthorized)
	}))
	defer server.Close()

	apiClient, err := New(server.URL, server.Client(), authenticator)
	if err != nil {
		t.Fatalf("create API client: %v", err)
	}
	err = apiClient.GetJSON(context.Background(), "/api/test", nil, &struct{}{})
	if err == nil {
		t.Fatal("GET JSON succeeded, want an error")
	}
	if strings.Contains(err.Error(), token) {
		t.Fatalf("error exposes JWT: %q", err)
	}
}
