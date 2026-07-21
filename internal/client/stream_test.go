package client

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/hugobrenet/opensvc-daemon-mcp/internal/auth"
)

func TestGetStream(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodGet || request.URL.Path != "/api/container/log" {
			t.Errorf("got %s %s, want GET /api/container/log", request.Method, request.URL.Path)
		}
		if got := request.URL.Query().Get("rid"); got != "container#redis" {
			t.Errorf("got rid %q, want container#redis", got)
		}
		if got := request.Header.Get("Accept"); got != "text/event-stream" {
			t.Errorf("got Accept %q, want text/event-stream", got)
		}
		if got := request.Header.Get("Authorization"); got != "Bearer delegated-token" {
			t.Errorf("got Authorization %q, want delegated token", got)
		}
		response.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprint(response, "redis ready\naccepting connections\n")
	}))
	defer server.Close()

	apiClient, err := New(server.URL, server.Client())
	if err != nil {
		t.Fatalf("create API client: %v", err)
	}
	var output bytes.Buffer
	ctx := auth.WithBearerToken(context.Background(), "delegated-token")
	err = apiClient.GetStream(ctx, "/api/container/log", url.Values{"rid": {"container#redis"}}, func(chunk []byte) error {
		_, _ = output.Write(chunk)
		return nil
	})
	if err != nil {
		t.Fatalf("GET stream: %v", err)
	}
	if got := output.String(); got != "redis ready\naccepting connections\n" {
		t.Errorf("got stream %q", got)
	}
}

func TestGetStreamHTTPError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, _ *http.Request) {
		response.Header().Set("Content-Type", "application/problem+json")
		response.WriteHeader(http.StatusForbidden)
		fmt.Fprint(response, `{"title":"Forbidden","detail":"need one of [root] grant"}`)
	}))
	defer server.Close()

	apiClient, _ := New(server.URL, server.Client())
	ctx := auth.WithBearerToken(context.Background(), "delegated-token")
	err := apiClient.GetStream(ctx, "/api/container/log", nil, func([]byte) error { return nil })
	var apiError *APIError
	if !errors.As(err, &apiError) || apiError.StatusCode != http.StatusForbidden || apiError.Detail != "need one of [root] grant" {
		t.Fatalf("got error %#v, want bounded APIError", err)
	}
}

func TestGetStreamRejectsUnexpectedContentType(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, _ *http.Request) {
		response.Header().Set("Content-Type", "text/plain")
		fmt.Fprint(response, "logs")
	}))
	defer server.Close()

	apiClient, _ := New(server.URL, server.Client())
	ctx := auth.WithBearerToken(context.Background(), "delegated-token")
	err := apiClient.GetStream(ctx, "/api/container/log", nil, func([]byte) error { return nil })
	if err == nil || !strings.Contains(err.Error(), "unexpected content type") {
		t.Fatalf("got error %v, want content type error", err)
	}
}

func TestGetStreamBoundsResponseAndPropagatesConsumerError(t *testing.T) {
	t.Run("response", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, _ *http.Request) {
			response.Header().Set("Content-Type", "text/event-stream")
			fmt.Fprint(response, strings.Repeat("x", maxStreamResponseBodySize+1))
		}))
		defer server.Close()

		apiClient, _ := New(server.URL, server.Client())
		ctx := auth.WithBearerToken(context.Background(), "delegated-token")
		err := apiClient.GetStream(ctx, "/api/container/log", nil, func([]byte) error { return nil })
		if err == nil || !strings.Contains(err.Error(), "exceeds") {
			t.Fatalf("got error %v, want oversized response error", err)
		}
	})

	t.Run("consumer", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, _ *http.Request) {
			response.Header().Set("Content-Type", "text/event-stream")
			fmt.Fprint(response, "logs")
		}))
		defer server.Close()

		apiClient, _ := New(server.URL, server.Client())
		ctx := auth.WithBearerToken(context.Background(), "delegated-token")
		want := errors.New("stop")
		err := apiClient.GetStream(ctx, "/api/container/log", nil, func([]byte) error { return want })
		if !errors.Is(err, want) {
			t.Fatalf("got error %v, want consumer error", err)
		}
	})
}

func TestGetStreamProcessesDataBeforeReadError(t *testing.T) {
	body := &dataAndErrorReadCloser{data: []byte("final log block"), err: io.ErrUnexpectedEOF}
	httpClient := &http.Client{Transport: streamRoundTripper(func(*http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     http.Header{"Content-Type": {"text/event-stream"}},
			Body:       body,
		}, nil
	})}
	apiClient, err := New("https://daemon.example", httpClient)
	if err != nil {
		t.Fatalf("create API client: %v", err)
	}
	ctx := auth.WithBearerToken(context.Background(), "delegated-token")
	var output bytes.Buffer
	err = apiClient.GetStream(ctx, "/api/container/log", nil, func(chunk []byte) error {
		_, _ = output.Write(chunk)
		return nil
	})
	if !errors.Is(err, io.ErrUnexpectedEOF) {
		t.Fatalf("got error %v, want unexpected EOF", err)
	}
	if got := output.String(); got != "final log block" {
		t.Fatalf("got consumed data %q, want final log block", got)
	}
}

type streamRoundTripper func(*http.Request) (*http.Response, error)

func (f streamRoundTripper) RoundTrip(request *http.Request) (*http.Response, error) {
	return f(request)
}

type dataAndErrorReadCloser struct {
	data []byte
	err  error
	done bool
}

func (r *dataAndErrorReadCloser) Read(buffer []byte) (int, error) {
	if r.done {
		return 0, io.EOF
	}
	r.done = true
	return copy(buffer, r.data), r.err
}

func (r *dataAndErrorReadCloser) Close() error {
	return nil
}
