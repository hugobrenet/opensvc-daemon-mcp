package client

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHTTPClientTLSVerification(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(response http.ResponseWriter, _ *http.Request) {
		response.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	verifiedResponse, err := NewHTTPClient(false).Get(server.URL)
	if err == nil {
		verifiedResponse.Body.Close()
		t.Fatal("verified TLS request succeeded with an untrusted certificate")
	}

	insecureResponse, err := NewHTTPClient(true).Get(server.URL)
	if err != nil {
		t.Fatalf("explicitly insecure TLS request failed: %v", err)
	}
	defer insecureResponse.Body.Close()
	if insecureResponse.StatusCode != http.StatusNoContent {
		t.Errorf("got HTTP status %s, want 204 No Content", insecureResponse.Status)
	}
}
