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
	"strings"
	"sync/atomic"
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

	var instanceRefreshed atomic.Bool
	daemonServer := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		if got := request.Header.Get("Authorization"); got != "Bearer "+token {
			http.Error(response, "unauthorized", http.StatusUnauthorized)
			return
		}
		response.Header().Set("Content-Type", "application/json")
		switch request.URL.Path {
		case "/api/cluster/status":
			fmt.Fprint(response, `{
			"cluster": {
				"config": {"id": "cluster-123", "name": "prod", "nodes": ["node-a"]},
				"status": {"is_compat": true, "is_frozen": false},
				"node": {"node-a": {
					"status": {"agent": "v3.0.0", "is_leader": true, "frozen_at": "0001-01-01T00:00:00Z"},
					"monitor": {"state": "idle"}
				}},
				"object": {"prod/svc/app": {
					"avail": "up", "overall": "up", "provisioned": "true",
					"frozen": "unfrozen", "placement_state": "optimal", "up_instances_count": 1,
					"scope": ["node-a"]
				}}
			},
			"daemon": {"nodename": "node-a"}
		}`)
		case "/api/object/path":
			if got := request.URL.Query().Get("path"); got != "**" {
				t.Errorf("got object selector %q, want **", got)
			}
			fmt.Fprint(response, `["prod/svc/app", "cluster"]`)
		case "/api/object":
			if got := request.URL.Query().Get("path"); got != "prod/svc/app" {
				t.Errorf("got object path %q, want prod/svc/app", got)
			}
			fmt.Fprint(response, `{"kind":"ObjectList","items":[{"kind":"ObjectItem","meta":{"object":"prod/svc/app"},"data":{"avail":"up","overall":"up","provisioned":"true","frozen":"unfrozen","placement_state":"optimal","placement_policy":"nodes order","orchestrate":"ha","topology":"failover","priority":50,"scope":["node-a"],"updated_at":"2026-07-15T10:00:00Z","up_instances_count":1,"instances":{"node-a":{}}}}]}`)
		case "/api/instance":
			if request.Method != http.MethodGet {
				t.Errorf("got instance method %q, want GET", request.Method)
			}
			if got := request.URL.Query().Get("path"); got != "prod/svc/app" {
				t.Errorf("got instance object path %q, want prod/svc/app", got)
			}
			updatedAt := "2026-07-15T10:00:00Z"
			if instanceRefreshed.Load() {
				updatedAt = "2026-07-15T10:00:01Z"
			}
			fmt.Fprintf(response, `{"kind":"InstanceList","items":[{"kind":"InstanceItem","meta":{"node":"node-a","object":"prod/svc/app"},"data":{"monitor":{"state":"idle","global_expect":"started","local_expect":"none","is_ha_leader":true,"orchestration_is_done":true},"status":{"avail":"up","overall":"up","provisioned":"true","updated_at":%q,"resources":{"app#1":{"status":"up"}}}}}]}`, updatedAt)
		case "/api/node/name/node-a/instance/path/prod/svc/app/action/status":
			if request.Method != http.MethodPost {
				t.Errorf("got refresh method %q, want POST", request.Method)
			}
			instanceRefreshed.Store(true)
			fmt.Fprint(response, `{"session_id":"session-1"}`)
		case "/api/resource":
			if got := request.URL.Query().Get("path"); got != "prod/svc/app" {
				t.Errorf("got resource object path %q, want prod/svc/app", got)
			}
			fmt.Fprint(response, `{"kind":"ResourceList","items":[{"kind":"ResourceItem","meta":{"node":"node-a","object":"prod/svc/app","rid":"app#1"},"data":{"config":{"is_disabled":false,"is_monitored":true,"is_standby":false},"status":{"type":"app.forking","label":"application","status":"up","monitor":true,"provisioned":{"state":"true"},"tags":[],"log":[]}}}]}`)
		default:
			t.Errorf("got unexpected daemon path %q", request.URL.Path)
			http.NotFound(response, request)
		}
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
	expectedToolTitles := map[string]string{
		"get_daemon_identity":     "Get daemon identity",
		"get_cluster_health":      "Assess cluster health",
		"get_object_status":       "Get object status",
		"list_cluster_objects":    "List cluster objects",
		"list_object_instances":   "List object instances",
		"list_object_resources":   "List object resources",
		"refresh_instance_status": "Refresh instance status",
	}
	toolNames := make(map[string]bool, len(availableTools.Tools))
	for _, tool := range availableTools.Tools {
		toolNames[tool.Name] = true
		if tool.Title != expectedToolTitles[tool.Name] {
			t.Errorf("tool %q has title %q, want %q", tool.Name, tool.Title, expectedToolTitles[tool.Name])
		}
		if tool.Description == "" {
			t.Errorf("tool %q has no description", tool.Name)
		}
		if tool.OutputSchema == nil {
			t.Errorf("tool %q has no output schema", tool.Name)
		}
		assertSchemaPropertyDescriptions(t, tool.OutputSchema, "outputSchema")
		if tool.Annotations == nil {
			t.Errorf("tool %q has no annotations", tool.Name)
		} else if tool.Name == "refresh_instance_status" {
			if tool.Annotations.ReadOnlyHint {
				t.Errorf("tool %q is incorrectly annotated as read-only", tool.Name)
			}
			if tool.Annotations.IdempotentHint {
				t.Errorf("tool %q is incorrectly annotated as idempotent", tool.Name)
			}
		} else if !tool.Annotations.ReadOnlyHint {
			t.Errorf("tool %q is not annotated as read-only", tool.Name)
		}
		if tool.Annotations != nil {
			if tool.Annotations.DestructiveHint == nil || *tool.Annotations.DestructiveHint {
				t.Errorf("tool %q is not explicitly annotated as non-destructive", tool.Name)
			}
			if tool.Annotations.OpenWorldHint == nil || *tool.Annotations.OpenWorldHint {
				t.Errorf("tool %q is not explicitly annotated as closed-world", tool.Name)
			}
		}
	}
	if len(toolNames) != len(expectedToolTitles) {
		t.Fatalf("got tools %#v, want exactly %#v", availableTools.Tools, expectedToolTitles)
	}
	for name := range expectedToolTitles {
		if !toolNames[name] {
			t.Errorf("tool %q is not registered", name)
		}
	}

	result, err := session.CallTool(ctx, &mcp.CallToolParams{
		Name:      "get_daemon_identity",
		Arguments: mcptools.GetDaemonIdentityInput{},
	})
	if err != nil {
		t.Fatalf("call get_daemon_identity: %v", err)
	}
	if result.IsError {
		t.Fatalf("get_daemon_identity returned an MCP tool error: %#v", result.Content)
	}

	data, err := json.Marshal(result.StructuredContent)
	if err != nil {
		t.Fatalf("marshal structured content: %v", err)
	}
	var identity mcptools.GetDaemonIdentityOutput
	if err := json.Unmarshal(data, &identity); err != nil {
		t.Fatalf("decode structured content: %v", err)
	}
	if identity.Daemon.NodeName != "node-a" || identity.Cluster.ID != "cluster-123" || identity.Node.AgentVersion != "v3.0.0" {
		t.Errorf("got unexpected identity %#v", identity)
	}

	result, err = session.CallTool(ctx, &mcp.CallToolParams{
		Name:      "get_cluster_health",
		Arguments: mcptools.GetClusterHealthInput{},
	})
	if err != nil {
		t.Fatalf("call get_cluster_health: %v", err)
	}
	if result.IsError {
		t.Fatalf("get_cluster_health returned an MCP tool error: %#v", result.Content)
	}
	data, err = json.Marshal(result.StructuredContent)
	if err != nil {
		t.Fatalf("marshal cluster health structured content: %v", err)
	}
	var health mcptools.GetClusterHealthOutput
	if err := json.Unmarshal(data, &health); err != nil {
		t.Fatalf("decode cluster health structured content: %v", err)
	}
	if !health.Healthy || health.ObjectSummary.Total != 1 || health.ObjectSummary.Up != 1 {
		t.Errorf("got unexpected cluster health %#v", health)
	}

	result, err = session.CallTool(ctx, &mcp.CallToolParams{
		Name:      "list_cluster_objects",
		Arguments: mcptools.ListClusterObjectsInput{},
	})
	if err != nil {
		t.Fatalf("call list_cluster_objects: %v", err)
	}
	if result.IsError {
		t.Fatalf("list_cluster_objects returned an MCP tool error: %#v", result.Content)
	}
	data, err = json.Marshal(result.StructuredContent)
	if err != nil {
		t.Fatalf("marshal cluster object list structured content: %v", err)
	}
	var objects mcptools.ListClusterObjectsOutput
	if err := json.Unmarshal(data, &objects); err != nil {
		t.Fatalf("decode cluster object list structured content: %v", err)
	}
	if objects.Total != 2 || objects.Count != 2 || objects.Objects[0].Path != "cluster" || objects.Objects[1].Path != "prod/svc/app" {
		t.Errorf("got unexpected cluster object list %#v", objects)
	}

	result, err = session.CallTool(ctx, &mcp.CallToolParams{
		Name:      "get_object_status",
		Arguments: mcptools.GetObjectStatusInput{Path: "prod/svc/app"},
	})
	if err != nil || result.IsError {
		t.Fatalf("call get_object_status: err=%v result=%#v", err, result)
	}
	data, _ = json.Marshal(result.StructuredContent)
	var objectStatus mcptools.GetObjectStatusOutput
	if err := json.Unmarshal(data, &objectStatus); err != nil {
		t.Fatalf("decode object status: %v", err)
	}
	if objectStatus.Availability != "up" || objectStatus.InstanceCount != 1 {
		t.Errorf("got unexpected object status %#v", objectStatus)
	}

	result, err = session.CallTool(ctx, &mcp.CallToolParams{
		Name:      "list_object_instances",
		Arguments: mcptools.ListObjectInstancesInput{Path: "prod/svc/app"},
	})
	if err != nil || result.IsError {
		t.Fatalf("call list_object_instances: err=%v result=%#v", err, result)
	}
	data, _ = json.Marshal(result.StructuredContent)
	var instances mcptools.ListObjectInstancesOutput
	if err := json.Unmarshal(data, &instances); err != nil {
		t.Fatalf("decode object instances: %v", err)
	}
	if instances.Count != 1 || instances.Instances[0].Node != "node-a" {
		t.Errorf("got unexpected object instances %#v", instances)
	}

	result, err = session.CallTool(ctx, &mcp.CallToolParams{
		Name: "refresh_instance_status",
		Arguments: mcptools.RefreshInstanceStatusInput{
			Path: "prod/svc/app", Node: "node-a", TimeoutSeconds: 5,
		},
	})
	if err != nil || result.IsError {
		t.Fatalf("call refresh_instance_status: err=%v result=%#v", err, result)
	}
	data, _ = json.Marshal(result.StructuredContent)
	var refreshed mcptools.RefreshInstanceStatusOutput
	if err := json.Unmarshal(data, &refreshed); err != nil {
		t.Fatalf("decode refreshed instance status: %v", err)
	}
	if !refreshed.RefreshObserved || refreshed.TimedOut || refreshed.SessionID != "session-1" || refreshed.CurrentUpdatedAt != "2026-07-15T10:00:01Z" {
		t.Errorf("got unexpected refresh result %#v", refreshed)
	}

	result, err = session.CallTool(ctx, &mcp.CallToolParams{
		Name:      "list_object_resources",
		Arguments: mcptools.ListObjectResourcesInput{Path: "prod/svc/app"},
	})
	if err != nil || result.IsError {
		t.Fatalf("call list_object_resources: err=%v result=%#v", err, result)
	}
	data, _ = json.Marshal(result.StructuredContent)
	var resources mcptools.ListObjectResourcesOutput
	if err := json.Unmarshal(data, &resources); err != nil {
		t.Fatalf("decode object resources: %v", err)
	}
	if resources.Count != 1 || resources.Resources[0].RID != "app#1" {
		t.Errorf("got unexpected object resources %#v", resources)
	}
}

func assertSchemaPropertyDescriptions(t *testing.T, schema any, path string) {
	t.Helper()
	schemaObject, ok := schema.(map[string]any)
	if !ok {
		t.Errorf("%s has type %T, want a JSON object", path, schema)
		return
	}

	if propertiesValue, exists := schemaObject["properties"]; exists {
		properties, ok := propertiesValue.(map[string]any)
		if !ok {
			t.Errorf("%s.properties has type %T, want a JSON object", path, propertiesValue)
			return
		}
		for name, propertyValue := range properties {
			property, ok := propertyValue.(map[string]any)
			propertyPath := path + ".properties." + name
			if !ok {
				t.Errorf("%s has type %T, want a JSON object", propertyPath, propertyValue)
				continue
			}
			description, _ := property["description"].(string)
			if strings.TrimSpace(description) == "" {
				t.Errorf("%s has no description", propertyPath)
			}
			assertSchemaPropertyDescriptions(t, property, propertyPath)
		}
	}

	if items, exists := schemaObject["items"]; exists {
		assertSchemaPropertyDescriptions(t, items, path+".items")
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
