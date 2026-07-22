# CODEX

Project guidance for AI coding agents working on opensvc-daemon-mcp.

## Mission

Build a small, secure, typed MCP server for the OpenSVC v3 daemon API.

The long-term goal is to support an AI operations agent that can inspect, diagnose, and eventually operate OpenSVC clusters. The MCP server is the deterministic integration layer. It must not contain autonomous decision-making logic.

## Current scope

The current implementation exposes a limited diagnostic tool surface. Most
tools are read-only; `refresh_instance_status` is an explicit, non-destructive
active probe. Tool contracts, endpoints, examples, and domain-specific behavior
are documented by domain in:

- [Tool index and diagnostic workflow](docs/tools/README.md)
- [Daemon tools](docs/tools/daemon.md)
- [Cluster tools](docs/tools/cluster.md)
- [Object tools](docs/tools/objects.md)
- [Instance tools](docs/tools/instances.md)
- [Resource tools](docs/tools/resources.md)

Streamable HTTP and delegated OpenSVC access JWT authentication are implemented. Every MCP request requires a Bearer token, the middleware validates it, and the same request-scoped token authenticates the tool's daemon API calls. Do not add additional tools, authentication modes, configuration frameworks, or generated API clients unless the user explicitly expands the scope.

## Technology

- Language: Go
- Minimum current toolchain: Go 1.25.5
- MCP SDK: github.com/modelcontextprotocol/go-sdk
- MCP transport: Streamable HTTP on loopback
- HTTP client: Go standard library
- Tests: Go testing and httptest

Prefer the Go standard library and keep dependencies minimal.

## Repository layout

~~~text
cmd/
  opensvc-daemon-mcp/
    main.go
    main_test.go

docs/
  tools/
    README.md
    daemon.md
    cluster.md
    objects.md
    instances.md
    resources.md

internal/
  auth/
    context.go
    context_test.go
    jwt.go
    jwt_test.go
    middleware.go
    middleware_test.go
  client/
    client.go
    client_test.go
    http.go
    http_test.go
    sse.go
    sse_test.go
    stream_test.go
  config/
    config.go
    config_test.go
  core/
    daemon.go
    daemon_test.go
    cluster.go
    cluster_test.go
    object.go
    object_test.go
    object_config.go
    object_config_test.go
    object_status.go
    object_status_test.go
    instance.go
    instance_test.go
    instance_logs.go
    instance_logs_test.go
    container_logs.go
    container_logs_test.go
    resource.go
    resource_test.go
  tools/
    annotations.go
    daemon.go
    cluster.go
    object.go
    instance.go
    resource.go
~~~

Do not reintroduce an internal/mcpserver package. The MCP server is intentionally created in main.go, similarly to the existing Python Collector MCP server entrypoint.

Do not add a global models package without a demonstrated shared-model requirement.

## Architecture

The package dependency flow is:

~~~text
main
  -> config
  -> auth middleware and JWT verifier
  -> client -> delegated auth context
  -> tools -> core
~~~

### main

cmd/opensvc-daemon-mcp/main.go is the composition root.

Keep this package limited to main.go and its end-to-end main_test.go. Dependency factories and configuration parsing belong to their responsible internal packages.

It is responsible for:

- reading process configuration;
- creating the JWT verifier and authentication middleware;
- creating the HTTP client;
- creating the core service;
- creating the MCP server;
- explicitly registering each tool domain;
- starting the Streamable HTTP transport.

The expected registration style is:

~~~go
tools.RegisterDaemonTools(server, service)
tools.RegisterClusterTools(server, service)
tools.RegisterObjectTools(server, service)
tools.RegisterInstanceTools(server, service)
tools.RegisterResourceTools(server, service)
// tools.RegisterNodeTools(server, service)
~~~

Only uncomment or add a domain when that domain actually exists.

### config

internal/config owns environment-variable loading, defaults, parsing, and the exported process Config type. It enforces a loopback listen address while MCP server TLS is not implemented.

### client

internal/client is transport-only.

Client.NewHTTPClient constructs the standard HTTP client, timeout, server trust roots, and optional development-only TLS verification bypass. It must fail fast on invalid TLS CA files.

Client.GetJSON, Client.PostJSON, Client.GetSSE, and Client.GetStream are responsible for:

- resolving a path against the daemon base URL;
- encoding query parameters;
- sending the requested HTTP operation;
- applying the delegated Bearer token from request context;
- checking the HTTP status.

Client.GetJSON and Client.PostJSON set JSON request headers and decode bounded
JSON responses.

Client.GetSSE additionally validates `text/event-stream`, parses bounded SSE
framing, and invokes a caller-provided event consumer. It does not reconnect or
follow a stream after EOF. OpenSVC event payload interpretation remains in the
core package.

Client.GetStream validates the same media type but delivers bounded opaque
chunks for daemon endpoints, such as container logs, that declare SSE while
returning raw stream bytes. It does not interpret or retain application logs.

Non-success responses are returned as `client.APIError`. The HTTP status is
authoritative. Optional RFC 7807 `title` and `detail` fields are read with
strict size limits, normalized, and exposed to the authenticated MCP caller.
Never retain or expose raw response bodies, authorization headers, or JWTs in
an API error.

The client must not know about MCP tools or business use cases.

Do not add a generic MCP tool that exposes Client.GetJSON.

### auth

internal/auth owns the delegated authentication flow.

The JWT verifier loads an RSA public key from the configured OpenSVC cluster CA certificate or public-key file. It accepts only RS256 access tokens with valid expiration and non-empty `sub` and `iss` claims plus `token_use=access`.

The middleware uses the MCP SDK bearer-auth middleware so authenticated subjects are bound to MCP sessions. It also retains the raw token in a private request-context value for delegation. The token must never be stored globally, written to disk, logged, returned, or placed in MCP arguments.

The daemon client reads the delegated token from context and sets `Authorization: Bearer <jwt>`. The OpenSVC daemon independently verifies the token and enforces its grants.

Basic Auth and X.509 client authentication are intentionally unsupported. Do not reintroduce them as daemon authentication alternatives or as fallback credentials.

### delegated JWT flow

The agreed HTTP architecture is:

~~~text
AI agent
  -> Authorization: Bearer <OpenSVC access JWT>
  -> MCP HTTP authentication middleware
  -> request context
  -> MCP tool
  -> OpenSVC API client with the same JWT
  -> OpenSVC daemon authorization
~~~

The middleware validates the RS256 signature using the public certificate of the OpenSVC cluster CA, requires a valid expiration and `token_use=access`, and exposes the authenticated subject and grants through MCP token metadata. The raw token remains request-scoped and must never be exposed to the model.

The MCP must not accept Basic Auth or X.509 client authentication as caller authentication. It must not fall back to a token file or service credential when a caller JWT is absent, invalid, expired, or unauthorized.

### core

internal/core owns OpenSVC semantics and use cases.

Core responsibilities include:

- selecting the exact OpenSVC API endpoint for a use case;
- validating domain inputs and required daemon fields;
- interpreting private endpoint-specific response shapes;
- filtering large daemon payloads;
- returning stable, typed, bounded business contracts.

Raw daemon API response types remain private to the core package. Tool-specific behavior belongs in the matching document under `docs/tools/`.

### tools

internal/tools owns MCP contracts and registration.

Each domain file exposes one registration function using Go exported naming:

~~~go
func RegisterDaemonTools(server *mcp.Server, service *core.Service)
func RegisterClusterTools(server *mcp.Server, service *core.Service)
func RegisterObjectTools(server *mcp.Server, service *core.Service)
func RegisterInstanceTools(server *mcp.Server, service *core.Service)
func RegisterResourceTools(server *mcp.Server, service *core.Service)
~~~

Tool handlers should remain thin:

1. accept typed MCP input;
2. call one core use case;
3. return typed structured output;
4. propagate useful errors.

Do not place HTTP paths, authentication logic, or response parsing in a tool handler.

Every tool declaration must expose a concise runtime contract through the MCP
protocol:

- a stable snake_case `Name` built from an explicit verb and domain noun;
- a short human-readable `Title`;
- a precise `Description` stating what the tool returns, when to use it, and
  an important limitation when applicable;
- typed input and output structures so the SDK publishes both JSON Schemas;
- standard `ToolAnnotations` that accurately describe its behavior.

Read-only tools contact only the configured OpenSVC daemon. Use
`readOnlyClosedWorldAnnotations` to declare them as read-only, non-destructive,
and closed-world. Active status probes use
`activeNonDestructiveClosedWorldAnnotations`: they are not read-only,
destructive, idempotent, or open-world. These annotations are hints for MCP
clients, not security controls. Authentication, OpenSVC grants, input
validation, and policy enforcement remain authoritative.

Do not publish proprietary runtime tags or custom `_meta` fields without a
concrete client interoperability requirement. The MCP SDK currently has no
standard tool-tags field. Documentation tags belong in the front matter of the
matching file under `docs/tools/`.

### tool documentation

Runtime MCP metadata is deliberately concise. The matching
`docs/tools/<domain>.md` file is the durable human and agent-facing contract.
Each domain document groups every tool owned by that domain and begins with
this front matter:

~~~yaml
---
domain: domain_name
tools:
  - first_tool_name
  - second_tool_name
stability: experimental
---
~~~

Update `stability` when the domain contracts mature. Do not use documentation
tags as authorization or runtime policy.

The domain index in `docs/tools/README.md` must list every registered tool and
show the recommended cross-domain diagnostic workflow. Each domain document
must introduce its core and MCP implementation files. Each documented tool
must cover:

- when to use the tool and when not to use it;
- MCP title, side-effect annotations, and their non-authoritative nature;
- delegated JWT authorization and visibility boundaries;
- exact OpenSVC endpoint and data freshness semantics;
- whether the endpoint reads last-known daemon state or actively refreshes drivers;
- input fields, defaults, validation, pagination, and selector behavior;
- every output field and any derived semantics;
- representative JSON input and complete bounded output examples captured from
  the lab and normalized for publication;
- expected authentication, authorization, transport, and data errors;
- the OpenSVC version against which behavior was verified.

Keep declarations, implementation, tests, and documentation synchronized in
the same change. The end-to-end `tools/list` test must assert the title,
description, output schema, and safety annotations of every registered tool.

## Type placement

Use these rules:

- MCP input/output types belong in internal/tools.
- Core business types belong in internal/core.
- Raw API response types belong privately in the layer that interprets them.
- Client transport types belong in internal/client.
- Types used by only one domain stay near that domain.
- Split a large file into another file in the same package before creating a generic models package.

Go imports are file-scoped. Files in the same package share declared types and functions, but they do not share imported package names.

## Go style

The repository owner is learning Go and already has strong C and Python experience.

Favor explicit, readable Go:

- avoid unnecessary generics;
- avoid reflection outside SDK behavior;
- avoid goroutines and channels unless concurrency is required;
- avoid dependency-injection frameworks;
- avoid premature interfaces;
- use context.Context for network and long-running operations;
- wrap errors with context using fmt.Errorf and %w;
- keep main small but explicit;
- prefer table-driven tests when multiple cases appear;
- use gofmt rather than manual formatting.

Explain non-obvious Go idioms when introducing them.

## Testing

Every change must preserve the complete vertical slice:

~~~text
MCP client with Bearer JWT
  -> Streamable HTTP authentication middleware
    -> MCP server
    -> tool handler
      -> core use case
        -> HTTP client
          -> fake OpenSVC daemon
~~~

Required validation:

~~~bash
go fmt ./...
go test -v ./...
go vet ./...
go build -o /tmp/opensvc-daemon-mcp ./cmd/opensvc-daemon-mcp
git diff --check
~~~

Keep unit tests beside the package they test.

Use httptest.Server for HTTP behavior. Do not require a live OpenSVC daemon for normal unit tests.

The end-to-end Streamable HTTP test in cmd/opensvc-daemon-mcp/main_test.go must continue to:

- build and start the real MCP binary on a temporary loopback port;
- sign a test access JWT and send it on every MCP request;
- list tools;
- call every registered tool;
- validate structured output;
- validate bounded daemon API errors through real MCP tool calls, including
  malformed, oversized, and interrupted responses.

## API and security rules

The client supports only delegated OpenSVC access JWTs received from authenticated MCP requests. Secrets must never enter MCP tool arguments or results.

Do not silently disable TLS certificate verification.

Future authentication material must remain outside tool input and output. Language models must never receive daemon tokens, passwords, or private keys.

The delegated JWT identifies the caller and lets the daemon enforce its OpenSVC grants. Until tool-specific policy and audit are designed:

- keep active operations limited to the explicit, non-destructive instance status refresh;
- do not add lifecycle, configuration, provisioning, or other state-changing actions;
- document live-daemon limitations;
- return explicit HTTP errors;
- do not work around a 401 or 403 by weakening security.

The daemon response may contain private configuration and large operational state. Return only fields required by the tool contract.

## Tool design rules

Before adding a tool:

1. start from an operational use case;
2. identify the exact OpenSVC API endpoint;
3. define a bounded typed output;
4. decide which fields are safe and useful for an LLM;
5. implement the core use case;
6. register the tool explicitly in main.go;
7. add unit and end-to-end coverage.
8. update the matching domain document under `docs/tools/`.

Avoid one-to-one exposure of every OpenAPI operation.

Avoid arbitrary path, method, or body parameters in MCP tools.

Lifecycle and configuration-changing tools will require a separate design for caller identity, authorization, confirmation, audit, idempotency, orchestration tracking, and post-action verification.

## Configuration

Current environment:

| Variable | Default |
|---|---|
| OPENSVC_DAEMON_URL | https://127.0.0.1:1215 |
| OPENSVC_DAEMON_REQUEST_TIMEOUT | 20s |
| OPENSVC_MCP_LISTEN_ADDRESS | 127.0.0.1:8080 |
| OPENSVC_MCP_JWT_VERIFY_KEY_FILE | /var/lib/opensvc/certs/ca_certificates |
| OPENSVC_DAEMON_TLS_CA_FILE | empty |
| OPENSVC_DAEMON_TLS_INSECURE | false |

Do not add configuration libraries for a small number of settings. Prefer the standard library until configuration complexity justifies another dependency.

`OPENSVC_DAEMON_TLS_INSECURE=true` is an explicit development-only escape hatch for local self-signed daemon certificates. It must remain disabled by default and must never be enabled silently.

## Dependency policy

Before adding a Go module:

- explain why the standard library is insufficient;
- prefer official or widely maintained packages;
- pin it through go.mod;
- run go mod tidy;
- review transitive dependencies;
- update README.md if installation or runtime behavior changes.

## Change discipline

- Preserve user changes and unrelated work.
- Keep changes scoped to the active request.
- Do not commit or push unless explicitly requested.
- Do not add build artifacts to Git.
- Do not store credentials in the repository.
- Keep README.md and CODEX.md aligned with the actual implementation.
- Keep tool contracts, endpoints, examples, and domain behavior in docs/tools.
- Update the matching domain document whenever a tool changes.
- Update tests whenever contracts or layer boundaries change.

## Known limitations

- no MCP server-side TLS yet, so listening is restricted to loopback;
- JWT creation and refresh remain the agent's responsibility;
- no Unix socket transport;
- a limited, mostly read-only diagnostic tool set;
- no tool-specific policy engine;
- no audit subsystem;
- no live OpenSVC integration test in the default test suite.

These are explicit project milestones, not reasons to bypass security.
