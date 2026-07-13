package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/hugobrenet/opensvc-daemon-mcp/internal/client"
	"github.com/hugobrenet/opensvc-daemon-mcp/internal/core"
	"github.com/hugobrenet/opensvc-daemon-mcp/internal/tools"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

const (
	serverName       = "opensvc-daemon-mcp"
	serverVersion    = "v0.1.0"
	defaultDaemonURL = "https://127.0.0.1:1215"
)

func main() {
	daemonURL := os.Getenv("OPENSVC_DAEMON_URL")
	if daemonURL == "" {
		daemonURL = defaultDaemonURL
	}

	apiClient, err := client.New(daemonURL, &http.Client{Timeout: 20 * time.Second})
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
