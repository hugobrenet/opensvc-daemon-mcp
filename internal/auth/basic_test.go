package auth

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestBasicApply(t *testing.T) {
	passwordFile := writePasswordFile(t, " secret password \r\n")
	authenticator, err := NewBasic("operator", passwordFile)
	if err != nil {
		t.Fatalf("create Basic authenticator: %v", err)
	}

	request, err := http.NewRequest(http.MethodGet, "https://127.0.0.1:1215/api/test", nil)
	if err != nil {
		t.Fatalf("create request: %v", err)
	}
	if err := authenticator.Apply(request); err != nil {
		t.Fatalf("apply Basic authentication: %v", err)
	}

	username, password, ok := request.BasicAuth()
	if !ok {
		t.Fatal("request has no Basic Authorization header")
	}
	if username != "operator" {
		t.Errorf("got username %q, want operator", username)
	}
	if password != " secret password " {
		t.Errorf("got password %q, want surrounding spaces to be preserved", password)
	}
}

func TestBasicApplyReadsRotatedPassword(t *testing.T) {
	passwordFile := writePasswordFile(t, "first-password")
	authenticator, err := NewBasic("operator", passwordFile)
	if err != nil {
		t.Fatalf("create Basic authenticator: %v", err)
	}

	firstRequest, err := http.NewRequest(http.MethodGet, "https://127.0.0.1:1215/api/test", nil)
	if err != nil {
		t.Fatalf("create first request: %v", err)
	}
	if err := authenticator.Apply(firstRequest); err != nil {
		t.Fatalf("apply first password: %v", err)
	}

	if err := os.WriteFile(passwordFile, []byte("second-password\n"), 0o600); err != nil {
		t.Fatalf("rotate password file: %v", err)
	}
	secondRequest, err := http.NewRequest(http.MethodGet, "https://127.0.0.1:1215/api/test", nil)
	if err != nil {
		t.Fatalf("create second request: %v", err)
	}
	if err := authenticator.Apply(secondRequest); err != nil {
		t.Fatalf("apply rotated password: %v", err)
	}

	_, firstPassword, _ := firstRequest.BasicAuth()
	_, secondPassword, _ := secondRequest.BasicAuth()
	if firstPassword != "first-password" {
		t.Errorf("got first password %q, want first-password", firstPassword)
	}
	if secondPassword != "second-password" {
		t.Errorf("got second password %q, want second-password", secondPassword)
	}
}

func TestBasicApplyRejectsMissingOrEmptyPasswordFile(t *testing.T) {
	tests := []struct {
		name         string
		passwordFile func(*testing.T) string
		wantError    string
	}{
		{
			name: "missing",
			passwordFile: func(t *testing.T) string {
				return filepath.Join(t.TempDir(), "missing-password")
			},
			wantError: "read OpenSVC daemon Basic Auth password file",
		},
		{
			name: "empty",
			passwordFile: func(t *testing.T) string {
				return writePasswordFile(t, "\n")
			},
			wantError: "is empty",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			authenticator, err := NewBasic("operator", test.passwordFile(t))
			if err != nil {
				t.Fatalf("create Basic authenticator: %v", err)
			}
			request, err := http.NewRequest(http.MethodGet, "https://127.0.0.1:1215/api/test", nil)
			if err != nil {
				t.Fatalf("create request: %v", err)
			}

			err = authenticator.Apply(request)
			if err == nil {
				t.Fatal("apply Basic authentication succeeded, want an error")
			}
			if !strings.Contains(err.Error(), test.wantError) {
				t.Fatalf("got error %q, want it to contain %q", err, test.wantError)
			}
			if got := request.Header.Get("Authorization"); got != "" {
				t.Errorf("got unexpected Authorization header %q", got)
			}
		})
	}
}

func TestNewBasicRejectsInvalidConfiguration(t *testing.T) {
	tests := []struct {
		name         string
		username     string
		passwordFile string
	}{
		{name: "empty username", username: " ", passwordFile: "/run/password"},
		{name: "empty password file", username: "operator", passwordFile: " "},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if _, err := NewBasic(test.username, test.passwordFile); err == nil {
				t.Fatal("NewBasic succeeded, want an error")
			}
		})
	}
}

func writePasswordFile(t *testing.T, password string) string {
	t.Helper()

	passwordFile := filepath.Join(t.TempDir(), "daemon.password")
	if err := os.WriteFile(passwordFile, []byte(password), 0o600); err != nil {
		t.Fatalf("write password file: %v", err)
	}
	return passwordFile
}
