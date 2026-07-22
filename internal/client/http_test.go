package client

import (
	"encoding/pem"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestHTTPClientTLSVerification(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(response http.ResponseWriter, _ *http.Request) {
		response.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	verifiedClient, err := NewHTTPClient(HTTPOptions{})
	if err != nil {
		t.Fatalf("create verified HTTP client: %v", err)
	}
	verifiedResponse, err := verifiedClient.Get(server.URL)
	if err == nil {
		verifiedResponse.Body.Close()
		t.Fatal("verified TLS request succeeded with an untrusted certificate")
	}

	insecureClient, err := NewHTTPClient(HTTPOptions{TLSInsecure: true})
	if err != nil {
		t.Fatalf("create explicitly insecure HTTP client: %v", err)
	}
	insecureResponse, err := insecureClient.Get(server.URL)
	if err != nil {
		t.Fatalf("explicitly insecure TLS request failed: %v", err)
	}
	defer insecureResponse.Body.Close()
	if insecureResponse.StatusCode != http.StatusNoContent {
		t.Errorf("got HTTP status %s, want 204 No Content", insecureResponse.Status)
	}
}

func TestHTTPClientCustomCA(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(response http.ResponseWriter, _ *http.Request) {
		response.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	certificatePEM := pem.EncodeToMemory(&pem.Block{
		Type:  "CERTIFICATE",
		Bytes: server.Certificate().Raw,
	})
	caFile := filepath.Join(t.TempDir(), "ca.crt")
	if err := os.WriteFile(caFile, certificatePEM, 0o600); err != nil {
		t.Fatalf("write test CA file: %v", err)
	}

	httpClient, err := NewHTTPClient(HTTPOptions{TLSCAFile: caFile})
	if err != nil {
		t.Fatalf("create HTTP client with custom CA: %v", err)
	}
	response, err := httpClient.Get(server.URL)
	if err != nil {
		t.Fatalf("request with custom CA: %v", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusNoContent {
		t.Errorf("got HTTP status %s, want 204 No Content", response.Status)
	}
}

func TestNewHTTPClientRejectsMissingCAFile(t *testing.T) {
	if _, err := NewHTTPClient(HTTPOptions{TLSCAFile: "/missing/ca.crt"}); err == nil {
		t.Fatal("NewHTTPClient succeeded, want an error")
	}
}

func TestHTTPClientRequestTimeout(t *testing.T) {
	requestStarted := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, request *http.Request) {
		close(requestStarted)
		<-request.Context().Done()
	}))
	defer server.Close()

	httpClient, err := NewHTTPClient(HTTPOptions{Timeout: 50 * time.Millisecond})
	if err != nil {
		t.Fatalf("create HTTP client: %v", err)
	}
	_, err = httpClient.Get(server.URL)
	if err == nil || !strings.Contains(err.Error(), "Client.Timeout exceeded") {
		t.Fatalf("got error %v, want client timeout", err)
	}
	select {
	case <-requestStarted:
	default:
		t.Fatal("server did not receive the timed out request")
	}
}

func TestHTTPClientTimeoutConfiguration(t *testing.T) {
	defaultClient, err := NewHTTPClient(HTTPOptions{})
	if err != nil {
		t.Fatalf("create default HTTP client: %v", err)
	}
	if defaultClient.Timeout != DefaultRequestTimeout {
		t.Fatalf("default timeout = %s, want %s", defaultClient.Timeout, DefaultRequestTimeout)
	}

	customClient, err := NewHTTPClient(HTTPOptions{Timeout: 45 * time.Second})
	if err != nil {
		t.Fatalf("create custom HTTP client: %v", err)
	}
	if customClient.Timeout != 45*time.Second {
		t.Fatalf("custom timeout = %s, want 45s", customClient.Timeout)
	}

	if _, err := NewHTTPClient(HTTPOptions{Timeout: -time.Second}); err == nil {
		t.Fatal("negative timeout succeeded")
	}
}
