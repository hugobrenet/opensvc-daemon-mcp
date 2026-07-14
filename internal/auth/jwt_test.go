package auth

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestJWTApply(t *testing.T) {
	tokenFile := writeTokenFile(t, "header.payload.signature\n")
	authenticator, err := NewJWT(tokenFile)
	if err != nil {
		t.Fatalf("create JWT authenticator: %v", err)
	}

	request, err := http.NewRequest(http.MethodGet, "https://127.0.0.1:1215/api/test", nil)
	if err != nil {
		t.Fatalf("create request: %v", err)
	}
	if err := authenticator.Apply(request); err != nil {
		t.Fatalf("apply JWT authentication: %v", err)
	}

	if got := request.Header.Get("Authorization"); got != "Bearer header.payload.signature" {
		t.Errorf("got Authorization header %q, want Bearer token", got)
	}
}

func TestJWTApplyReadsRotatedToken(t *testing.T) {
	tokenFile := writeTokenFile(t, "first-token")
	authenticator, err := NewJWT(tokenFile)
	if err != nil {
		t.Fatalf("create JWT authenticator: %v", err)
	}

	firstRequest, err := http.NewRequest(http.MethodGet, "https://127.0.0.1:1215/api/test", nil)
	if err != nil {
		t.Fatalf("create first request: %v", err)
	}
	if err := authenticator.Apply(firstRequest); err != nil {
		t.Fatalf("apply first JWT: %v", err)
	}

	if err := os.WriteFile(tokenFile, []byte("second-token\n"), 0o600); err != nil {
		t.Fatalf("rotate token file: %v", err)
	}
	secondRequest, err := http.NewRequest(http.MethodGet, "https://127.0.0.1:1215/api/test", nil)
	if err != nil {
		t.Fatalf("create second request: %v", err)
	}
	if err := authenticator.Apply(secondRequest); err != nil {
		t.Fatalf("apply rotated JWT: %v", err)
	}

	if got := firstRequest.Header.Get("Authorization"); got != "Bearer first-token" {
		t.Errorf("got first Authorization header %q, want first token", got)
	}
	if got := secondRequest.Header.Get("Authorization"); got != "Bearer second-token" {
		t.Errorf("got second Authorization header %q, want rotated token", got)
	}
}

func TestJWTApplyRejectsMissingOrEmptyTokenFile(t *testing.T) {
	tests := []struct {
		name      string
		tokenFile func(*testing.T) string
		wantError string
	}{
		{
			name: "missing",
			tokenFile: func(t *testing.T) string {
				return filepath.Join(t.TempDir(), "missing-token")
			},
			wantError: "read OpenSVC daemon JWT file",
		},
		{
			name: "empty",
			tokenFile: func(t *testing.T) string {
				return writeTokenFile(t, " \n\t")
			},
			wantError: "is empty",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			authenticator, err := NewJWT(test.tokenFile(t))
			if err != nil {
				t.Fatalf("create JWT authenticator: %v", err)
			}
			request, err := http.NewRequest(http.MethodGet, "https://127.0.0.1:1215/api/test", nil)
			if err != nil {
				t.Fatalf("create request: %v", err)
			}

			err = authenticator.Apply(request)
			if err == nil {
				t.Fatal("apply JWT authentication succeeded, want an error")
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

func TestNewJWTRejectsEmptyPath(t *testing.T) {
	_, err := NewJWT(" \t")
	if err == nil {
		t.Fatal("NewJWT succeeded, want an error")
	}
}

func writeTokenFile(t *testing.T, token string) string {
	t.Helper()

	tokenFile := filepath.Join(t.TempDir(), "daemon.jwt")
	if err := os.WriteFile(tokenFile, []byte(token), 0o600); err != nil {
		t.Fatalf("write token file: %v", err)
	}
	return tokenFile
}
