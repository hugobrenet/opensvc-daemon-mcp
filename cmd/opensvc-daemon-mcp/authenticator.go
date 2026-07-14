package main

import (
	"fmt"

	"github.com/hugobrenet/opensvc-daemon-mcp/internal/auth"
)

func newAuthenticator(method string, tokenFile string) (auth.Authenticator, error) {
	switch method {
	case "jwt":
		return auth.NewJWT(tokenFile)
	case "none":
		return auth.None{}, nil
	default:
		return nil, fmt.Errorf("unsupported OpenSVC daemon authentication method %q", method)
	}
}
