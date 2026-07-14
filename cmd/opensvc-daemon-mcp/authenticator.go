package main

import (
	"fmt"

	"github.com/hugobrenet/opensvc-daemon-mcp/internal/auth"
)

func newAuthenticator(cfg authConfig) (auth.Authenticator, error) {
	switch cfg.method {
	case "jwt":
		return auth.NewJWT(cfg.tokenFile)
	case "basic":
		return auth.NewBasic(cfg.basicUsername, cfg.basicPasswordFile)
	case "none":
		return auth.None{}, nil
	default:
		return nil, fmt.Errorf("unsupported OpenSVC daemon authentication method %q", cfg.method)
	}
}
