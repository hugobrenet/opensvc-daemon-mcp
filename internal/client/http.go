package client

import (
	"crypto/tls"
	"net/http"
	"time"
)

// NewHTTPClient constructs the HTTP client used to contact the OpenSVC daemon.
func NewHTTPClient(tlsInsecure bool) *http.Client {
	client := &http.Client{Timeout: 20 * time.Second}
	if !tlsInsecure {
		return client
	}

	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.TLSClientConfig = &tls.Config{
		InsecureSkipVerify: true, // Explicit development-only escape hatch for self-signed daemon certificates.
	}
	client.Transport = transport
	return client
}
