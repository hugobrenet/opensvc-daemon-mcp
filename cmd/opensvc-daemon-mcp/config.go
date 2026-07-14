package main

import (
	"fmt"
	"os"
	"strconv"
)

const (
	defaultDaemonURL   = "https://127.0.0.1:1215"
	defaultAuthMethod  = "jwt"
	defaultTokenFile   = "/run/opensvc-daemon-mcp/token"
	defaultTLSInsecure = false
)

type config struct {
	daemonURL   string
	authMethod  string
	tokenFile   string
	tlsInsecure bool
}

func loadConfig() (config, error) {
	tlsInsecure, err := strconv.ParseBool(
		getenv("OPENSVC_DAEMON_TLS_INSECURE", strconv.FormatBool(defaultTLSInsecure)),
	)
	if err != nil {
		return config{}, fmt.Errorf("parse OPENSVC_DAEMON_TLS_INSECURE: %w", err)
	}

	return config{
		daemonURL:   getenv("OPENSVC_DAEMON_URL", defaultDaemonURL),
		authMethod:  getenv("OPENSVC_DAEMON_AUTH_METHOD", defaultAuthMethod),
		tokenFile:   getenv("OPENSVC_DAEMON_TOKEN_FILE", defaultTokenFile),
		tlsInsecure: tlsInsecure,
	}, nil
}

func getenv(name string, fallback string) string {
	if value := os.Getenv(name); value != "" {
		return value
	}
	return fallback
}
