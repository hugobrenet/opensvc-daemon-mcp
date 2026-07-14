package config

import (
	"fmt"
	"net"
	"os"
	"strconv"

	"github.com/hugobrenet/opensvc-daemon-mcp/internal/client"
)

const (
	defaultDaemonURL        = "https://127.0.0.1:1215"
	defaultListenAddress    = "127.0.0.1:8080"
	defaultJWTVerifyKeyFile = "/var/lib/opensvc/certs/ca_certificates"
	defaultTLSInsecure      = false
)

// Config contains the runtime configuration of the MCP server process.
type Config struct {
	DaemonURL        string
	ListenAddress    string
	JWTVerifyKeyFile string
	HTTP             client.HTTPOptions
}

// Load reads and validates process configuration from environment variables.
func Load() (Config, error) {
	tlsInsecure, err := strconv.ParseBool(
		getenv("OPENSVC_DAEMON_TLS_INSECURE", strconv.FormatBool(defaultTLSInsecure)),
	)
	if err != nil {
		return Config{}, fmt.Errorf("parse OPENSVC_DAEMON_TLS_INSECURE: %w", err)
	}
	listenAddress := getenv("OPENSVC_MCP_LISTEN_ADDRESS", defaultListenAddress)
	if err := validateLoopbackAddress(listenAddress); err != nil {
		return Config{}, fmt.Errorf("validate OPENSVC_MCP_LISTEN_ADDRESS: %w", err)
	}
	return Config{
		DaemonURL:        getenv("OPENSVC_DAEMON_URL", defaultDaemonURL),
		ListenAddress:    listenAddress,
		JWTVerifyKeyFile: getenv("OPENSVC_MCP_JWT_VERIFY_KEY_FILE", defaultJWTVerifyKeyFile),
		HTTP: client.HTTPOptions{
			TLSInsecure: tlsInsecure,
			TLSCAFile:   os.Getenv("OPENSVC_DAEMON_TLS_CA_FILE"),
		},
	}, nil
}

func validateLoopbackAddress(address string) error {
	host, port, err := net.SplitHostPort(address)
	if err != nil {
		return fmt.Errorf("split host and port: %w", err)
	}
	ip := net.ParseIP(host)
	if ip == nil || !ip.IsLoopback() {
		return fmt.Errorf("HTTP MCP transport without TLS must listen on a loopback IP")
	}
	portNumber, err := strconv.Atoi(port)
	if err != nil || portNumber < 0 || portNumber > 65535 {
		return fmt.Errorf("invalid TCP port %q", port)
	}
	return nil
}

func getenv(name string, fallback string) string {
	if value := os.Getenv(name); value != "" {
		return value
	}
	return fallback
}
