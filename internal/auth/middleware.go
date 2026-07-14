package auth

import (
	"net/http"
	"strings"

	mcpauth "github.com/modelcontextprotocol/go-sdk/auth"
)

// Middleware requires a valid Bearer token and keeps it in the request context
// for delegation to the OpenSVC daemon API.
func Middleware(verifier mcpauth.TokenVerifier) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		protected := mcpauth.RequireBearerToken(verifier, nil)(next)
		return http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
			if token, ok := bearerToken(request.Header.Get("Authorization")); ok {
				request = request.WithContext(WithBearerToken(request.Context(), token))
			}
			protected.ServeHTTP(response, request)
		})
	}
}

func bearerToken(authorization string) (string, bool) {
	fields := strings.Fields(authorization)
	if len(fields) != 2 || !strings.EqualFold(fields[0], "Bearer") {
		return "", false
	}
	return fields[1], true
}
