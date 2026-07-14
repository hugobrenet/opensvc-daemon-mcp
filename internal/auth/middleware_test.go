package auth

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	mcpauth "github.com/modelcontextprotocol/go-sdk/auth"
)

func TestMiddlewareAuthenticatesAndCapturesBearerToken(t *testing.T) {
	verifier := func(_ context.Context, token string, _ *http.Request) (*mcpauth.TokenInfo, error) {
		if token != "valid-token" {
			return nil, mcpauth.ErrInvalidToken
		}
		return &mcpauth.TokenInfo{
			UserID:     "operator",
			Expiration: time.Now().Add(time.Hour),
		}, nil
	}

	handler := Middleware(verifier)(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		token, ok := BearerTokenFromContext(request.Context())
		if !ok || token != "valid-token" {
			t.Errorf("got delegated token %q, present=%v", token, ok)
		}
		info := mcpauth.TokenInfoFromContext(request.Context())
		if info == nil || info.UserID != "operator" {
			t.Errorf("got token info %#v, want operator", info)
		}
		response.WriteHeader(http.StatusNoContent)
	}))

	request := httptest.NewRequest(http.MethodPost, "/mcp", nil)
	request.Header.Set("Authorization", "Bearer valid-token")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)

	if response.Code != http.StatusNoContent {
		t.Errorf("got HTTP status %d, want 204", response.Code)
	}
}

func TestMiddlewareRejectsMissingOrInvalidBearerToken(t *testing.T) {
	verifier := func(_ context.Context, _ string, _ *http.Request) (*mcpauth.TokenInfo, error) {
		return nil, errors.Join(mcpauth.ErrInvalidToken, errors.New("verification failed"))
	}
	handler := Middleware(verifier)(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		t.Fatal("protected handler was called")
	}))

	for _, test := range []struct {
		name          string
		authorization string
	}{
		{name: "missing"},
		{name: "Basic", authorization: "Basic dXNlcjpwYXNzd29yZA=="},
		{name: "invalid JWT", authorization: "Bearer invalid-token"},
	} {
		t.Run(test.name, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodPost, "/mcp", nil)
			request.Header.Set("Authorization", test.authorization)
			response := httptest.NewRecorder()

			handler.ServeHTTP(response, request)
			if response.Code != http.StatusUnauthorized {
				t.Errorf("got HTTP status %d, want 401", response.Code)
			}
		})
	}
}
