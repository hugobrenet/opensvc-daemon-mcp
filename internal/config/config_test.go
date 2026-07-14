package config

import (
	"strings"
	"testing"

	"github.com/hugobrenet/opensvc-daemon-mcp/internal/auth"
	"github.com/hugobrenet/opensvc-daemon-mcp/internal/client"
)

func TestLoadDefaults(t *testing.T) {
	t.Setenv("OPENSVC_DAEMON_URL", "")
	t.Setenv("OPENSVC_DAEMON_AUTH_METHOD", "")
	t.Setenv("OPENSVC_DAEMON_TOKEN_FILE", "")
	t.Setenv("OPENSVC_DAEMON_BASIC_USERNAME", "")
	t.Setenv("OPENSVC_DAEMON_BASIC_PASSWORD_FILE", "")
	t.Setenv("OPENSVC_DAEMON_X509_CERT_FILE", "")
	t.Setenv("OPENSVC_DAEMON_X509_KEY_FILE", "")
	t.Setenv("OPENSVC_DAEMON_TLS_CA_FILE", "")
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
		HTTP: client.HTTPOptions{TLSInsecure: defaultTLSInsecure},
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
	t.Setenv("OPENSVC_DAEMON_X509_CERT_FILE", "/tmp/client.crt")
	t.Setenv("OPENSVC_DAEMON_X509_KEY_FILE", "/tmp/client.key")
	t.Setenv("OPENSVC_DAEMON_TLS_CA_FILE", "/tmp/ca.crt")
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
		HTTP: client.HTTPOptions{
			TLSInsecure:              true,
			TLSCAFile:                "/tmp/ca.crt",
			TLSClientCertificateFile: "/tmp/client.crt",
			TLSClientKeyFile:         "/tmp/client.key",
		},
	}
	if got != want {
		t.Errorf("got config %#v, want %#v", got, want)
	}
}

func TestLoadX509Defaults(t *testing.T) {
	t.Setenv("OPENSVC_DAEMON_AUTH_METHOD", "x509")
	t.Setenv("OPENSVC_DAEMON_X509_CERT_FILE", "")
	t.Setenv("OPENSVC_DAEMON_X509_KEY_FILE", "")

	got, err := Load()
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	if got.HTTP.TLSClientCertificateFile != defaultX509CertFile {
		t.Errorf("got certificate file %q, want %q", got.HTTP.TLSClientCertificateFile, defaultX509CertFile)
	}
	if got.HTTP.TLSClientKeyFile != defaultX509KeyFile {
		t.Errorf("got key file %q, want %q", got.HTTP.TLSClientKeyFile, defaultX509KeyFile)
	}
}

func TestLoadRejectsIncompleteX509Pair(t *testing.T) {
	t.Setenv("OPENSVC_DAEMON_X509_CERT_FILE", "/tmp/client.crt")
	t.Setenv("OPENSVC_DAEMON_X509_KEY_FILE", "")

	_, err := Load()
	if err == nil {
		t.Fatal("Load succeeded, want an error")
	}
	if !strings.Contains(err.Error(), "configured together") {
		t.Fatalf("got error %q, want certificate/key configuration error", err)
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
