package main

import (
	"context"
	"log"

	"github.com/hugobrenet/opensvc-daemon-mcp/internal/client"
	"github.com/hugobrenet/opensvc-daemon-mcp/internal/core"
	"github.com/hugobrenet/opensvc-daemon-mcp/internal/tools"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

const (
	serverName    = "opensvc-daemon-mcp"
	serverVersion = "v0.1.0"
)

func main() {
	cfg, err := loadConfig()
	if err != nil {
		log.Fatal(err)
	}

	authenticator, err := newAuthenticator(
		cfg.authMethod,
		cfg.tokenFile,
	)
	if err != nil {
		log.Fatal(err)
	}

	httpClient := newHTTPClient(cfg.tlsInsecure)

	apiClient, err := client.New(cfg.daemonURL, httpClient, authenticator)
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
