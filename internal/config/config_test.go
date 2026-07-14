package config

import (
	"strings"
	"testing"

	"github.com/hugobrenet/opensvc-daemon-mcp/internal/auth"
)

func TestLoadDefaults(t *testing.T) {
	t.Setenv("OPENSVC_DAEMON_URL", "")
	t.Setenv("OPENSVC_DAEMON_AUTH_METHOD", "")
	t.Setenv("OPENSVC_DAEMON_TOKEN_FILE", "")
	t.Setenv("OPENSVC_DAEMON_BASIC_USERNAME", "")
	t.Setenv("OPENSVC_DAEMON_BASIC_PASSWORD_FILE", "")
	t.Setenv("OPENSVC_DAEMON_TLS_INSECURE", "")

	got, err := Load()
	if err != nil {
		t.Fatalf("load config: %v", err)
	}

	want := Config{
		DaemonURL: defaultDaemonURL,
		Auth: auth.Options{
			Method:            defaultAuthMethod,
			TokenFile:         defaultTokenFile,
			BasicUsername:     "",
			BasicPasswordFile: defaultBasicPasswordFile,
		},
		TLSInsecure: defaultTLSInsecure,
	}
	if got != want {
		t.Errorf("got config %#v, want %#v", got, want)
	}
}

func TestLoadFromEnvironment(t *testing.T) {
	t.Setenv("OPENSVC_DAEMON_URL", "https://node-a.example:1215")
	t.Setenv("OPENSVC_DAEMON_AUTH_METHOD", "none")
	t.Setenv("OPENSVC_DAEMON_TOKEN_FILE", "/tmp/daemon.jwt")
	t.Setenv("OPENSVC_DAEMON_BASIC_USERNAME", "operator")
	t.Setenv("OPENSVC_DAEMON_BASIC_PASSWORD_FILE", "/tmp/daemon.password")
	t.Setenv("OPENSVC_DAEMON_TLS_INSECURE", "true")

	got, err := Load()
	if err != nil {
		t.Fatalf("load config: %v", err)
	}

	want := Config{
		DaemonURL: "https://node-a.example:1215",
		Auth: auth.Options{
			Method:            "none",
			TokenFile:         "/tmp/daemon.jwt",
			BasicUsername:     "operator",
			BasicPasswordFile: "/tmp/daemon.password",
		},
		TLSInsecure: true,
	}
	if got != want {
		t.Errorf("got config %#v, want %#v", got, want)
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
