package main

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"math/big"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	mcptools "github.com/hugobrenet/opensvc-daemon-mcp/internal/tools"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestServerOverStreamableHTTP(t *testing.T) {
	privateKey, verifyKeyFile := writeTestJWTVerifyKey(t)
	token := signJWT(t, privateKey, jwt.MapClaims{
		"exp":       time.Now().Add(time.Hour).Unix(),
		"grant":     []string{"guest"},
		"iss":       "node-a",
		"sub":       "test-user",
		"token_use": "access",
	})

	daemonServer := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/api/cluster/status" {
			t.Errorf("got daemon path %q, want /api/cluster/status", request.URL.Path)
		}
		if got := request.Header.Get("Authorization"); got != "Bearer "+token {
			http.Error(response, "unauthorized", http.StatusUnauthorized)
			return
		}
		response.Header().Set("Content-Type", "application/json")
		fmt.Fprint(response, `{
			"cluster": {
				"config": {"id": "cluster-123", "name": "prod"},
				"node": {"node-a": {"status": {"agent": "v3.0.0"}}}
			},
			"daemon": {"nodename": "node-a"}
		}`)
	}))
	defer daemonServer.Close()

	listenAddress := reserveLoopbackAddress(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	binary := filepath.Join(t.TempDir(), "opensvc-daemon-mcp")
	build := exec.CommandContext(ctx, "go", "build", "-o", binary, ".")
	if output, err := build.CombinedOutput(); err != nil {
		cancel()
		t.Fatalf("build MCP server: %v\n%s", err, output)
	}
	command := exec.CommandContext(ctx, binary)
	command.Env = append(
		os.Environ(),
		"OPENSVC_DAEMON_URL="+daemonServer.URL,
		"OPENSVC_MCP_LISTEN_ADDRESS="+listenAddress,
		"OPENSVC_MCP_JWT_VERIFY_KEY_FILE="+verifyKeyFile,
	)
	var serverOutput bytes.Buffer
	command.Stdout = &serverOutput
	command.Stderr = &serverOutput
	if err := command.Start(); err != nil {
		cancel()
		t.Fatalf("start MCP server: %v", err)
	}
	defer func() {
		cancel()
		_ = command.Wait()
	}()

	httpClient := &http.Client{Transport: bearerRoundTripper{
		base:  http.DefaultTransport,
		token: token,
	}}
	var session *mcp.ClientSession
	var err error
	deadline := time.Now().Add(15 * time.Second)
	for time.Now().Before(deadline) {
		mcpClient := mcp.NewClient(
			&mcp.Implementation{Name: "opensvc-daemon-mcp-test", Version: "v0.1.0"},
			nil,
		)
		session, err = mcpClient.Connect(ctx, &mcp.StreamableClientTransport{
			Endpoint:             "http://" + listenAddress + "/mcp",
			HTTPClient:           httpClient,
			DisableStandaloneSSE: true,
		}, nil)
		if err == nil {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}
	if err != nil {
		t.Fatalf("connect to MCP server: %v\nserver output:\n%s", err, serverOutput.String())
	}
	defer func() {
		if err := session.Close(); err != nil {
			t.Errorf("close MCP session: %v", err)
		}
	}()

	availableTools, err := session.ListTools(ctx, &mcp.ListToolsParams{})
	if err != nil {
		t.Fatalf("list MCP tools: %v", err)
	}
	if len(availableTools.Tools) != 1 || availableTools.Tools[0].Name != "get_server_identity" {
		t.Fatalf("got tools %#v, want get_server_identity", availableTools.Tools)
	}

	result, err := session.CallTool(ctx, &mcp.CallToolParams{
		Name:      "get_server_identity",
		Arguments: mcptools.GetServerIdentityInput{},
	})
	if err != nil {
		t.Fatalf("call get_server_identity: %v", err)
	}
	if result.IsError {
		t.Fatalf("get_server_identity returned an MCP tool error: %#v", result.Content)
	}

	data, err := json.Marshal(result.StructuredContent)
	if err != nil {
		t.Fatalf("marshal structured content: %v", err)
	}
	var identity mcptools.GetServerIdentityOutput
	if err := json.Unmarshal(data, &identity); err != nil {
		t.Fatalf("decode structured content: %v", err)
	}
	if identity.Daemon.NodeName != "node-a" || identity.Cluster.ID != "cluster-123" || identity.Node.AgentVersion != "v3.0.0" {
		t.Errorf("got unexpected identity %#v", identity)
	}
}

type bearerRoundTripper struct {
	base  http.RoundTripper
	token string
}

func (t bearerRoundTripper) RoundTrip(request *http.Request) (*http.Response, error) {
	clone := request.Clone(request.Context())
	clone.Header = request.Header.Clone()
	clone.Header.Set("Authorization", "Bearer "+t.token)
	return t.base.RoundTrip(clone)
}

func reserveLoopbackAddress(t *testing.T) string {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("reserve loopback address: %v", err)
	}
	address := listener.Addr().String()
	if err := listener.Close(); err != nil {
		t.Fatalf("release loopback address: %v", err)
	}
	return address
}

func writeTestJWTVerifyKey(t *testing.T) (*rsa.PrivateKey, string) {
	t.Helper()
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generate RSA key: %v", err)
	}
	template := &x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{CommonName: "OpenSVC test CA"},
		NotBefore:             time.Now().Add(-time.Minute),
		NotAfter:              time.Now().Add(time.Hour),
		IsCA:                  true,
		BasicConstraintsValid: true,
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageDigitalSignature,
	}
	certificateDER, err := x509.CreateCertificate(rand.Reader, template, template, &privateKey.PublicKey, privateKey)
	if err != nil {
		t.Fatalf("create test certificate: %v", err)
	}
	verifyKeyFile := filepath.Join(t.TempDir(), "cluster-ca.pem")
	certificatePEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certificateDER})
	if err := os.WriteFile(verifyKeyFile, certificatePEM, 0o600); err != nil {
		t.Fatalf("write test certificate: %v", err)
	}
	return privateKey, verifyKeyFile
}

func signJWT(t *testing.T, privateKey *rsa.PrivateKey, claims jwt.MapClaims) string {
	t.Helper()
	token, err := jwt.NewWithClaims(jwt.SigningMethodRS256, claims).SignedString(privateKey)
	if err != nil {
		t.Fatalf("sign test JWT: %v", err)
	}
	return token
}
