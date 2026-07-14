package auth

import (
	"context"
	"fmt"
	"net/http"
)

type bearerTokenContextKey struct{}

// WithBearerToken attaches a delegated OpenSVC access token to a request context.
// The server authentication middleware is the only production caller.
func WithBearerToken(ctx context.Context, token string) context.Context {
	return context.WithValue(ctx, bearerTokenContextKey{}, token)
}

// BearerTokenFromContext returns the delegated OpenSVC access token.
func BearerTokenFromContext(ctx context.Context) (string, bool) {
	token, ok := ctx.Value(bearerTokenContextKey{}).(string)
	return token, ok && token != ""
}

// ApplyBearerFromContext forwards the delegated token to an OpenSVC daemon request.
func ApplyBearerFromContext(request *http.Request) error {
	token, ok := BearerTokenFromContext(request.Context())
	if !ok {
		return fmt.Errorf("delegated OpenSVC access JWT is missing from request context")
	}
	request.Header.Set("Authorization", "Bearer "+token)
	return nil
}
