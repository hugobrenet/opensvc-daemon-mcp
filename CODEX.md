# CODEX

Project guidance for AI coding agents working on opensvc-daemon-mcp.

## Mission

Build a small, secure, typed MCP server for the OpenSVC v3 daemon API.

The long-term goal is to support an AI operations agent that can inspect, diagnose, and eventually operate OpenSVC clusters. The MCP server is the deterministic integration layer. It must not contain autonomous decision-making logic.

## Current scope

The current scope is intentionally limited to one read-only tool:

~~~text
get_server_identity
~~~

This tool reads:

~~~text
GET /api/cluster/status?selector=**
~~~

and returns a filtered identity response for the local daemon, cluster, node, and listener.

JWT Bearer and Basic Auth from rotating secret files are implemented. Do not add additional tools, authentication modes, transports, configuration frameworks, or generated API clients unless the user explicitly expands the scope.

## Technology

- Language: Go
- Minimum current toolchain: Go 1.25.5
- MCP SDK: github.com/modelcontextprotocol/go-sdk
- MCP transport: stdio
- HTTP client: Go standard library
- Tests: Go testing and httptest

Prefer the Go standard library and keep dependencies minimal.

## Repository layout

~~~text
cmd/
  opensvc-daemon-mcp/
    authenticator.go
    config.go
    config_test.go
    http_client.go
    http_client_test.go
    main.go
    main_test.go

internal/
  auth/
    auth.go
    basic.go
    basic_test.go
    jwt.go
    jwt_test.go
  client/
    client.go
    client_test.go
  core/
    identity.go
    identity_test.go
  tools/
    identity.go
~~~

Do not reintroduce an internal/mcpserver package. The MCP server is intentionally created in main.go, similarly to the existing Python Collector MCP server entrypoint.

Do not add a global models package without a demonstrated shared-model requirement.

## Architecture

The dependency flow is:

~~~text
main
  -> tools
    -> core
      -> client
        -> auth
~~~

### main

cmd/opensvc-daemon-mcp/main.go is the composition root.

cmd/opensvc-daemon-mcp/config.go owns environment-variable loading, defaults, parsing, and the process configuration type.

cmd/opensvc-daemon-mcp/authenticator.go and http_client.go own executable-specific dependency construction. Keep environment parsing out of these files and keep transport/authentication implementation details out of main.go.

It is responsible for:

- reading process configuration;
- creating the selected daemon request authenticator;
- creating the HTTP client;
- creating the core service;
- creating the MCP server;
- explicitly registering each tool domain;
- starting the selected MCP transport.

The expected registration style is:

~~~go
tools.RegisterIdentityTools(server, service)
// tools.RegisterClusterTools(server, service)
// tools.RegisterNodeTools(server, service)
~~~

Only uncomment or add a domain when that domain actually exists.

### client

internal/client is transport-only.

Client.GetJSON is responsible for:

- resolving a path against the daemon base URL;
- encoding query parameters;
- sending an HTTP GET request;
- setting JSON request headers;
- applying the injected request authenticator;
- checking the HTTP status;
- decoding a bounded JSON response.

The client must not know about MCP tools or business use cases.

Do not add a generic MCP tool that exposes Client.GetJSON.

### auth

internal/auth owns request authentication only.

The Authenticator interface applies credentials to an HTTP request. The JWT implementation:

- reads the configured token file on every request;
- trims surrounding whitespace;
- sets `Authorization: Bearer <jwt>`;
- fails on a missing or empty file;
- never decodes, validates, returns, or logs the token.

JWT verification and grant enforcement belong to the OpenSVC daemon. Token creation and refresh are outside the current MCP server scope.

The Basic Auth implementation:

- reads the configured password file on every request;
- removes one trailing LF or CRLF line ending but preserves other whitespace;
- uses `http.Request.SetBasicAuth` rather than building the header manually;
- fails on a missing or empty username or password file;
- never returns or logs the password.

The `none` implementation is reserved for unit tests and fake unprotected daemons. Do not use it to bypass authentication on a real daemon.

Prefer a dedicated least-privileged OpenSVC `system/usr/<username>` object for Basic Auth. Do not use node-name plus cluster-secret authentication merely for convenience, because it grants root access.

### core

internal/core owns OpenSVC semantics and use cases.

Core responsibilities for get_server_identity include:

- selecting /api/cluster/status;
- setting selector=**;
- decoding the endpoint-specific shape;
- finding the local node from daemon.nodename;
- validating required identity fields;
- filtering the large cluster status payload;
- returning the stable ServerIdentity contract.

The raw cluster status response type remains private to the core package.

### tools

internal/tools owns MCP contracts and registration.

Each domain file exposes one registration function using Go exported naming:

~~~go
func RegisterIdentityTools(server *mcp.Server, service *core.Service)
~~~

Tool handlers should remain thin:

1. accept typed MCP input;
2. call one core use case;
3. return typed structured output;
4. propagate useful errors.

Do not place HTTP paths, authentication logic, or response parsing in a tool handler.

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
MCP client
  -> stdio MCP server
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

The end-to-end stdio test in cmd/opensvc-daemon-mcp/main_test.go must continue to:

- start the real MCP binary through go run;
- list tools;
- call get_server_identity;
- validate structured output.

## API and security rules

The current client supports JWT Bearer and Basic Auth, with JWT as the default. Secrets come only from configured files and must never enter MCP tool arguments or results.

Do not silently disable TLS certificate verification.

Future authentication material must remain outside tool input and output. Language models must never receive daemon tokens, passwords, private keys, or client certificates.

JWT authentication to the daemon does not provide authorization for the human or agent invoking the MCP server. Until caller authorization is designed:

- keep tools read-only;
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

Avoid one-to-one exposure of every OpenAPI operation.

Avoid arbitrary path, method, or body parameters in MCP tools.

State-changing tools will require a separate design for caller identity, authorization, confirmation, audit, idempotency, orchestration tracking, and post-action verification.

## Configuration

Current environment:

| Variable | Default |
|---|---|
| OPENSVC_DAEMON_URL | https://127.0.0.1:1215 |
| OPENSVC_DAEMON_AUTH_METHOD | jwt |
| OPENSVC_DAEMON_TOKEN_FILE | /run/opensvc-daemon-mcp/token |
| OPENSVC_DAEMON_BASIC_USERNAME | empty |
| OPENSVC_DAEMON_BASIC_PASSWORD_FILE | /run/opensvc-daemon-mcp/password |
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
- Update tests whenever contracts or layer boundaries change.

## Known limitations

- JWT and Basic Auth are the production daemon authentication methods;
- no automatic JWT creation or refresh;
- no X.509 client-certificate authentication;
- no custom CA or client certificate configuration;
- no Unix socket transport;
- no HTTP MCP transport;
- one tool only;
- no caller authorization or policy engine;
- no audit subsystem;
- no live OpenSVC integration test in the default test suite.

These are explicit project milestones, not reasons to bypass security.
