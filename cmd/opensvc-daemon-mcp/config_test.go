package main

import (
	"strings"
	"testing"
)

func TestLoadConfigDefaults(t *testing.T) {
	t.Setenv("OPENSVC_DAEMON_URL", "")
	t.Setenv("OPENSVC_DAEMON_AUTH_METHOD", "")
	t.Setenv("OPENSVC_DAEMON_TOKEN_FILE", "")
	t.Setenv("OPENSVC_DAEMON_BASIC_USERNAME", "")
	t.Setenv("OPENSVC_DAEMON_BASIC_PASSWORD_FILE", "")
	t.Setenv("OPENSVC_DAEMON_TLS_INSECURE", "")

	got, err := loadConfig()
	if err != nil {
		t.Fatalf("load config: %v", err)
	}

	want := config{
		daemonURL: defaultDaemonURL,
		auth: authConfig{
			method:            defaultAuthMethod,
			tokenFile:         defaultTokenFile,
			basicUsername:     "",
			basicPasswordFile: defaultBasicPasswordFile,
		},
		tlsInsecure: defaultTLSInsecure,
	}
	if got != want {
		t.Errorf("got config %#v, want %#v", got, want)
	}
}

func TestLoadConfigFromEnvironment(t *testing.T) {
	t.Setenv("OPENSVC_DAEMON_URL", "https://node-a.example:1215")
	t.Setenv("OPENSVC_DAEMON_AUTH_METHOD", "none")
	t.Setenv("OPENSVC_DAEMON_TOKEN_FILE", "/tmp/daemon.jwt")
	t.Setenv("OPENSVC_DAEMON_BASIC_USERNAME", "operator")
	t.Setenv("OPENSVC_DAEMON_BASIC_PASSWORD_FILE", "/tmp/daemon.password")
	t.Setenv("OPENSVC_DAEMON_TLS_INSECURE", "true")

	got, err := loadConfig()
	if err != nil {
		t.Fatalf("load config: %v", err)
	}

	want := config{
		daemonURL: "https://node-a.example:1215",
		auth: authConfig{
			method:            "none",
			tokenFile:         "/tmp/daemon.jwt",
			basicUsername:     "operator",
			basicPasswordFile: "/tmp/daemon.password",
		},
		tlsInsecure: true,
	}
	if got != want {
		t.Errorf("got config %#v, want %#v", got, want)
	}
}

func TestLoadConfigRejectsInvalidTLSInsecure(t *testing.T) {
	t.Setenv("OPENSVC_DAEMON_TLS_INSECURE", "sometimes")

	_, err := loadConfig()
	if err == nil {
		t.Fatal("loadConfig succeeded, want an error")
	}
	if !strings.Contains(err.Error(), "OPENSVC_DAEMON_TLS_INSECURE") {
		t.Fatalf("got error %q, want variable name", err)
	}
}
