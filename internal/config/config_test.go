package config

import (
	"strings"
	"testing"
	"time"

	"github.com/hugobrenet/opensvc-daemon-mcp/internal/client"
)

func TestLoadDefaults(t *testing.T) {
	t.Setenv("OPENSVC_DAEMON_URL", "")
	t.Setenv("OPENSVC_MCP_LISTEN_ADDRESS", "")
	t.Setenv("OPENSVC_MCP_JWT_VERIFY_KEY_FILE", "")
	t.Setenv("OPENSVC_DAEMON_TLS_CA_FILE", "")
	t.Setenv("OPENSVC_DAEMON_TLS_INSECURE", "")
	t.Setenv("OPENSVC_DAEMON_REQUEST_TIMEOUT", "")

	got, err := Load()
	if err != nil {
		t.Fatalf("load config: %v", err)
	}

	want := Config{
		DaemonURL:        defaultDaemonURL,
		ListenAddress:    defaultListenAddress,
		JWTVerifyKeyFile: defaultJWTVerifyKeyFile,
		HTTP: client.HTTPOptions{
			TLSInsecure: defaultTLSInsecure,
			Timeout:     client.DefaultRequestTimeout,
		},
	}
	if got != want {
		t.Errorf("got config %#v, want %#v", got, want)
	}
}

func TestLoadFromEnvironment(t *testing.T) {
	t.Setenv("OPENSVC_DAEMON_URL", "https://node-a.example:1215")
	t.Setenv("OPENSVC_MCP_LISTEN_ADDRESS", "127.0.0.1:9090")
	t.Setenv("OPENSVC_MCP_JWT_VERIFY_KEY_FILE", "/tmp/cluster-ca.pem")
	t.Setenv("OPENSVC_DAEMON_TLS_CA_FILE", "/tmp/ca.crt")
	t.Setenv("OPENSVC_DAEMON_TLS_INSECURE", "true")
	t.Setenv("OPENSVC_DAEMON_REQUEST_TIMEOUT", "45s")

	got, err := Load()
	if err != nil {
		t.Fatalf("load config: %v", err)
	}

	want := Config{
		DaemonURL:        "https://node-a.example:1215",
		ListenAddress:    "127.0.0.1:9090",
		JWTVerifyKeyFile: "/tmp/cluster-ca.pem",
		HTTP: client.HTTPOptions{
			TLSInsecure: true,
			TLSCAFile:   "/tmp/ca.crt",
			Timeout:     45 * time.Second,
		},
	}
	if got != want {
		t.Errorf("got config %#v, want %#v", got, want)
	}
}

func TestLoadRejectsNonLoopbackListenAddress(t *testing.T) {
	t.Setenv("OPENSVC_MCP_LISTEN_ADDRESS", "0.0.0.0:8080")

	_, err := Load()
	if err == nil {
		t.Fatal("Load succeeded, want an error")
	}
	if !strings.Contains(err.Error(), "loopback") {
		t.Fatalf("got error %q, want loopback restriction", err)
	}
}

func TestLoadRejectsInvalidTLSInsecure(t *testing.T) {
	t.Setenv("OPENSVC_DAEMON_TLS_INSECURE", "sometimes")

	_, err := Load()
	if err == nil {
		t.Fatal("Load succeeded, want an error")
	}
	if !strings.Contains(err.Error(), "OPENSVC_DAEMON_TLS_INSECURE") {
		t.Fatalf("got error %q, want variable name", err)
	}
}

func TestLoadRejectsInvalidDaemonRequestTimeout(t *testing.T) {
	for _, value := range []string{"invalid", "0s", "500ms", "2m1s"} {
		t.Run(value, func(t *testing.T) {
			t.Setenv("OPENSVC_DAEMON_REQUEST_TIMEOUT", value)
			_, err := Load()
			if err == nil {
				t.Fatal("Load succeeded, want an error")
			}
			if !strings.Contains(err.Error(), "OPENSVC_DAEMON_REQUEST_TIMEOUT") {
				t.Fatalf("got error %q, want variable name", err)
			}
		})
	}
}
