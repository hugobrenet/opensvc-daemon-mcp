package client

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"net/http"
	"os"
	"time"
)

// HTTPOptions configures TLS for the OpenSVC daemon HTTP client.
type HTTPOptions struct {
	TLSInsecure              bool
	TLSCAFile                string
	TLSClientCertificateFile string
	TLSClientKeyFile         string
}

// NewHTTPClient constructs the HTTP client used to contact the OpenSVC daemon.
func NewHTTPClient(options HTTPOptions) (*http.Client, error) {
	if (options.TLSClientCertificateFile == "") != (options.TLSClientKeyFile == "") {
		return nil, fmt.Errorf("OpenSVC daemon TLS client certificate and key must be configured together")
	}

	transport := http.DefaultTransport.(*http.Transport).Clone()
	tlsConfig := &tls.Config{
		InsecureSkipVerify: options.TLSInsecure, // Explicit development-only escape hatch for self-signed daemon certificates.
		MinVersion:         tls.VersionTLS12,
	}

	if options.TLSCAFile != "" {
		caPEM, err := os.ReadFile(options.TLSCAFile)
		if err != nil {
			return nil, fmt.Errorf("read OpenSVC daemon TLS CA file %q: %w", options.TLSCAFile, err)
		}
		rootCAs, err := x509.SystemCertPool()
		if err != nil {
			rootCAs = x509.NewCertPool()
		}
		if !rootCAs.AppendCertsFromPEM(caPEM) {
			return nil, fmt.Errorf("OpenSVC daemon TLS CA file %q contains no certificates", options.TLSCAFile)
		}
		tlsConfig.RootCAs = rootCAs
	}

	if options.TLSClientCertificateFile != "" {
		certificate, err := tls.LoadX509KeyPair(options.TLSClientCertificateFile, options.TLSClientKeyFile)
		if err != nil {
			return nil, fmt.Errorf("load OpenSVC daemon TLS client certificate and key: %w", err)
		}
		tlsConfig.Certificates = []tls.Certificate{certificate}
	}

	transport.TLSClientConfig = tlsConfig
	client := &http.Client{Transport: transport, Timeout: 20 * time.Second}
	return client, nil
}
