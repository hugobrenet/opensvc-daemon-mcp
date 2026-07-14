package auth

import (
	"strings"
	"testing"
)

func TestNew(t *testing.T) {
	tests := []struct {
		name    string
		options Options
		check   func(*testing.T, Authenticator)
	}{
		{
			name:    "JWT",
			options: Options{Method: "jwt", TokenFile: "/run/token"},
			check: func(t *testing.T, authenticator Authenticator) {
				if _, ok := authenticator.(*JWT); !ok {
					t.Fatalf("got authenticator %T, want *JWT", authenticator)
				}
			},
		},
		{
			name: "Basic",
			options: Options{
				Method:            "basic",
				BasicUsername:     "operator",
				BasicPasswordFile: "/run/password",
			},
			check: func(t *testing.T, authenticator Authenticator) {
				if _, ok := authenticator.(*Basic); !ok {
					t.Fatalf("got authenticator %T, want *Basic", authenticator)
				}
			},
		},
		{
			name:    "none",
			options: Options{Method: "none"},
			check: func(t *testing.T, authenticator Authenticator) {
				if _, ok := authenticator.(None); !ok {
					t.Fatalf("got authenticator %T, want None", authenticator)
				}
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			authenticator, err := New(test.options)
			if err != nil {
				t.Fatalf("create authenticator: %v", err)
			}
			test.check(t, authenticator)
		})
	}
}

func TestNewRejectsUnsupportedMethod(t *testing.T) {
	_, err := New(Options{Method: "unknown"})
	if err == nil {
		t.Fatal("New succeeded, want an error")
	}
	if !strings.Contains(err.Error(), "unknown") {
		t.Fatalf("got error %q, want method name", err)
	}
}
