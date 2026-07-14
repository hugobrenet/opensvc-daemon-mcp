package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/hugobrenet/opensvc-daemon-mcp/internal/auth"
	"github.com/hugobrenet/opensvc-daemon-mcp/internal/client"
	"github.com/hugobrenet/opensvc-daemon-mcp/internal/core"
	"github.com/hugobrenet/opensvc-daemon-mcp/internal/tools"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

const (
	serverName        = "opensvc-daemon-mcp"
	serverVersion     = "v0.1.0"
	defaultDaemonURL  = "https://127.0.0.1:1215"
	defaultAuthMethod = "jwt"
	defaultTokenFile  = "/run/opensvc-daemon-mcp/token"
)

func main() {
	daemonURL := os.Getenv("OPENSVC_DAEMON_URL")
	if daemonURL == "" {
		daemonURL = defaultDaemonURL
	}

	authenticator, err := newAuthenticator(
		getenv("OPENSVC_DAEMON_AUTH_METHOD", defaultAuthMethod),
		getenv("OPENSVC_DAEMON_TOKEN_FILE", defaultTokenFile),
	)
	if err != nil {
		log.Fatal(err)
	}

	apiClient, err := client.New(daemonURL, &http.Client{Timeout: 20 * time.Second}, authenticator)
	if err != nil {
		log.Fatal(err)
	}
	service := core.New(apiClient)

	server := mcp.NewServer(
		&mcp.Implementation{
			Name:    serverName,
			Version: serverVersion,
		},
		nil,
	)
	tools.RegisterIdentityTools(server, service)

	if err := server.Run(context.Background(), &mcp.StdioTransport{}); err != nil {
		log.Fatal(err)
	}
}

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

func getenv(name string, fallback string) string {
	if value := os.Getenv(name); value != "" {
		return value
	}
	return fallback
}
