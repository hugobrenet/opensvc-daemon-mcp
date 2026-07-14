package config

import (
	"fmt"
	"os"
	"strconv"

	"github.com/hugobrenet/opensvc-daemon-mcp/internal/auth"
	"github.com/hugobrenet/opensvc-daemon-mcp/internal/client"
)

const (
	defaultDaemonURL   = "https://127.0.0.1:1215"
	defaultAuthMethod  = "jwt"
	defaultTokenFile   = "/run/opensvc-daemon-mcp/token"
	defaultTLSInsecure = false
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
	return Config{
		DaemonURL: getenv("OPENSVC_DAEMON_URL", defaultDaemonURL),
		Auth: auth.Options{
			Method:    getenv("OPENSVC_DAEMON_AUTH_METHOD", defaultAuthMethod),
			TokenFile: getenv("OPENSVC_DAEMON_TOKEN_FILE", defaultTokenFile),
		},
		HTTP: client.HTTPOptions{
			TLSInsecure: tlsInsecure,
			TLSCAFile:   os.Getenv("OPENSVC_DAEMON_TLS_CA_FILE"),
		},
	}, nil
}

func getenv(name string, fallback string) string {
	if value := os.Getenv(name); value != "" {
		return value
	}
	return fallback
}
