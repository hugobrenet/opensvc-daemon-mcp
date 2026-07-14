package main

import (
	"context"
	"log"

	"github.com/hugobrenet/opensvc-daemon-mcp/internal/auth"
	"github.com/hugobrenet/opensvc-daemon-mcp/internal/client"
	"github.com/hugobrenet/opensvc-daemon-mcp/internal/config"
	"github.com/hugobrenet/opensvc-daemon-mcp/internal/core"
	"github.com/hugobrenet/opensvc-daemon-mcp/internal/tools"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

const (
	serverName    = "opensvc-daemon-mcp"
	serverVersion = "v0.1.0"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatal(err)
	}

	authenticator, err := auth.New(cfg.Auth)
	if err != nil {
		log.Fatal(err)
	}

	httpClient := client.NewHTTPClient(cfg.TLSInsecure)

	apiClient, err := client.New(cfg.DaemonURL, httpClient, authenticator)
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
