package main

import (
	"log"
	"net/http"
	"time"

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

	verifier, err := auth.NewJWTVerifier(cfg.JWTVerifyKeyFile)
	if err != nil {
		log.Fatal(err)
	}

	httpClient, err := client.NewHTTPClient(cfg.HTTP)
	if err != nil {
		log.Fatal(err)
	}

	apiClient, err := client.New(cfg.DaemonURL, httpClient)
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
	registrar, err := tools.NewRegistrar(server)
	if err != nil {
		log.Fatal(err)
	}
	if err := tools.RegisterDaemonTools(registrar, service); err != nil {
		log.Fatal(err)
	}
	if err := tools.RegisterClusterTools(registrar, service); err != nil {
		log.Fatal(err)
	}
	if err := tools.RegisterObjectTools(registrar, service); err != nil {
		log.Fatal(err)
	}
	if err := tools.RegisterInstanceTools(registrar, service); err != nil {
		log.Fatal(err)
	}
	if err := tools.RegisterResourceTools(registrar, service); err != nil {
		log.Fatal(err)
	}

	streamHandler := mcp.NewStreamableHTTPHandler(
		func(*http.Request) *mcp.Server { return server },
		nil,
	)
	mux := http.NewServeMux()
	mux.Handle("/mcp", auth.Middleware(verifier.Verify)(streamHandler))

	httpServer := &http.Server{
		Addr:              cfg.ListenAddress,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
		IdleTimeout:       2 * time.Minute,
		MaxHeaderBytes:    64 << 10,
	}
	log.Printf("%s %s listening on http://%s/mcp", serverName, serverVersion, cfg.ListenAddress)
	if err := httpServer.ListenAndServe(); err != nil {
		log.Fatal(err)
	}
}
