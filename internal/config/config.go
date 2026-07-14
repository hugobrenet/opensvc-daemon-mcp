package config

import (
	"fmt"
	"os"
	"strconv"

	"github.com/hugobrenet/opensvc-daemon-mcp/internal/auth"
	"github.com/hugobrenet/opensvc-daemon-mcp/internal/client"
)

const (
	defaultDaemonURL         = "https://127.0.0.1:1215"
	defaultAuthMethod        = "jwt"
	defaultTokenFile         = "/run/opensvc-daemon-mcp/token"
	defaultBasicPasswordFile = "/run/opensvc-daemon-mcp/password"
	defaultX509CertFile      = "/run/opensvc-daemon-mcp/client.crt"
	defaultX509KeyFile       = "/run/opensvc-daemon-mcp/client.key"
	defaultTLSInsecure       = false
)

// Config contains the runtime configuration of the MCP server process.
type Config struct {
	DaemonURL string
	Auth      auth.Options
	HTTP      client.HTTPOptions
}

// Load reads and validates process configuration from environment variables.
func Load() (Config, error) {
	tlsInsecure, err := strconv.ParseBool(
		getenv("OPENSVC_DAEMON_TLS_INSECURE", strconv.FormatBool(defaultTLSInsecure)),
	)
	if err != nil {
		return Config{}, fmt.Errorf("parse OPENSVC_DAEMON_TLS_INSECURE: %w", err)
	}
	authMethod := getenv("OPENSVC_DAEMON_AUTH_METHOD", defaultAuthMethod)
	x509CertFile := os.Getenv("OPENSVC_DAEMON_X509_CERT_FILE")
	x509KeyFile := os.Getenv("OPENSVC_DAEMON_X509_KEY_FILE")
	if authMethod == "x509" {
		if x509CertFile == "" {
			x509CertFile = defaultX509CertFile
		}
		if x509KeyFile == "" {
			x509KeyFile = defaultX509KeyFile
		}
	}
	if (x509CertFile == "") != (x509KeyFile == "") {
		return Config{}, fmt.Errorf("OPENSVC_DAEMON_X509_CERT_FILE and OPENSVC_DAEMON_X509_KEY_FILE must be configured together")
	}

	return Config{
		DaemonURL: getenv("OPENSVC_DAEMON_URL", defaultDaemonURL),
		Auth: auth.Options{
			Method:            authMethod,
			TokenFile:         getenv("OPENSVC_DAEMON_TOKEN_FILE", defaultTokenFile),
			BasicUsername:     getenv("OPENSVC_DAEMON_BASIC_USERNAME", ""),
			BasicPasswordFile: getenv("OPENSVC_DAEMON_BASIC_PASSWORD_FILE", defaultBasicPasswordFile),
		},
		HTTP: client.HTTPOptions{
			TLSInsecure:              tlsInsecure,
			TLSCAFile:                os.Getenv("OPENSVC_DAEMON_TLS_CA_FILE"),
			TLSClientCertificateFile: x509CertFile,
			TLSClientKeyFile:         x509KeyFile,
		},
	}, nil
}

func getenv(name string, fallback string) string {
	if value := os.Getenv(name); value != "" {
		return value
	}
	return fallback
}
