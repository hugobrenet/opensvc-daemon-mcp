# OpenSVC Daemon MCP

A Go-based Model Context Protocol server that gives AI agents a controlled, typed interface to the OpenSVC v3 daemon API.

The project is intended to become the low-level operational MCP layer for AI-assisted inspection, diagnosis, and administration of OpenSVC clusters. One MCP server is expected to run close to each OpenSVC daemon and expose carefully designed tools instead of a generic raw API proxy.

## Project status

This project is in an early development stage.

The current implementation:

- runs as an MCP server over stdin/stdout;
- uses the official Go MCP SDK;
- connects to a configurable OpenSVC daemon API URL;
- authenticates daemon API requests with a JWT Bearer token read from a file;
- exposes one tool: get_server_identity;
- calls GET /api/cluster/status with selector=**;
- returns a filtered, structured identity response;
- reloads the token file for every request so the JWT can be rotated without restarting the MCP server;
- has no Basic Auth, client-certificate authentication, or custom TLS configuration yet.

It is not production-ready.

## Current tool

### get_server_identity

Returns identity information reported by the local OpenSVC daemon.

The tool has no input parameters.

Example structured output:

~~~json
{
  "daemon": {
    "nodename": "node-a",
    "pid": 2610,
    "started_at": "2026-07-10T17:23:35+09:00",
    "routines": 121
  },
  "cluster": {
    "id": "cluster-123",
    "name": "prod",
    "nodes": ["node-a", "node-b"],
    "quorum": true
  },
  "node": {
    "agent_version": "v3.0.0",
    "api_version": 1,
    "compat_version": 2,
    "is_leader": true,
    "is_overloaded": false,
    "booted_at": "2026-07-10T17:23:14+09:00"
  },
  "listener": {
    "address": "::",
    "port": 1215
  }
}
~~~

The full cluster status payload is intentionally not exposed. Object, instance, pool, schedule, heartbeat, and private node configuration data will be handled by dedicated tools if needed.

## Architecture

The project currently follows four simple layers:

~~~text
MCP tool
  -> core use case
    -> generic OpenSVC API client
      -> OpenSVC daemon API
~~~

Repository layout:

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

Responsibilities:

- cmd/opensvc-daemon-mcp/config.go loads and validates process configuration from environment variables.
- cmd/opensvc-daemon-mcp/authenticator.go selects the configured daemon request authenticator.
- cmd/opensvc-daemon-mcp/http_client.go constructs the daemon HTTP client and its TLS policy.
- cmd/opensvc-daemon-mcp/main.go builds the dependencies, creates the MCP server, registers tool domains, and starts the stdio transport.
- internal/auth applies the configured authentication method to daemon API requests.
- internal/client contains generic HTTP transport behavior for the OpenSVC daemon API.
- internal/core contains OpenSVC-specific use cases and response shaping.
- internal/tools contains MCP input/output contracts and tool registration.

The MCP layer does not expose a generic call_api tool. Every capability must have an explicit, bounded contract.

## Requirements

- Go 1.25.5 or later
- Access to an OpenSVC v3 daemon API and a valid daemon JWT
- Git

## Installation from source

Clone the repository:

~~~bash
git clone https://github.com/hugobrenet/opensvc-daemon-mcp.git
cd opensvc-daemon-mcp
~~~

Download dependencies:

~~~bash
go mod download
~~~

Build the server:

~~~bash
mkdir -p bin
go build -o bin/opensvc-daemon-mcp ./cmd/opensvc-daemon-mcp
~~~

## Configuration

The server supports these environment variables:

| Variable | Default | Description |
|---|---|---|
| OPENSVC_DAEMON_URL | https://127.0.0.1:1215 | Base URL of the local OpenSVC daemon API |
| OPENSVC_DAEMON_AUTH_METHOD | jwt | Daemon API authentication method. `none` is reserved for tests and fake daemons. |
| OPENSVC_DAEMON_TOKEN_FILE | /run/opensvc-daemon-mcp/token | File containing the raw JWT, without the `Bearer` prefix |
| OPENSVC_DAEMON_TLS_INSECURE | false | Disable daemon certificate verification. Development only. |

Example:

~~~bash
export OPENSVC_DAEMON_URL=https://127.0.0.1:1215
export OPENSVC_DAEMON_AUTH_METHOD=jwt
export OPENSVC_DAEMON_TOKEN_FILE=$HOME/.config/opensvc-daemon-mcp/daemon.jwt
~~~

For a local development daemon using a self-signed certificate, verification can be explicitly disabled:

~~~bash
export OPENSVC_DAEMON_TLS_INSECURE=true
~~~

This disables certificate-chain and hostname verification. Never enable it when connecting to a daemon over an untrusted network. The default remains secure.

The token file should be readable only by the MCP process owner. Its content is trimmed and sent as:

~~~text
Authorization: Bearer <jwt>
~~~

The MCP server does not decode or validate the JWT. The OpenSVC daemon validates it and applies its grants. The file is read for every API request, allowing an external process to rotate it atomically without restarting the MCP server.

For an unprotected fake daemon in development, authentication can be explicitly disabled:

~~~bash
export OPENSVC_DAEMON_AUTH_METHOD=none
~~~

Do not use `none` with a real daemon.

Basic Auth, client certificates, custom certificate authorities, and Unix socket transport are not implemented yet.

A protected or self-signed daemon endpoint may therefore reject the live request until those features are added.

## Run

The server currently uses MCP stdio transport:

~~~bash
OPENSVC_DAEMON_URL=https://127.0.0.1:1215 \
OPENSVC_DAEMON_TOKEN_FILE=$HOME/.config/opensvc-daemon-mcp/daemon.jwt \
OPENSVC_DAEMON_TLS_INSECURE=true \
  ./bin/opensvc-daemon-mcp
~~~

The process waits for MCP JSON-RPC messages on stdin and writes responses to stdout. It is normally started by an MCP client rather than used interactively.

## Development

Format the code:

~~~bash
go fmt ./...
~~~

Run tests:

~~~bash
go test -v ./...
~~~

Run static analysis:

~~~bash
go vet ./...
~~~

Build without writing a binary into the repository root:

~~~bash
go build -o /tmp/opensvc-daemon-mcp ./cmd/opensvc-daemon-mcp
~~~

The test suite covers:

- generic JSON GET requests;
- JWT Bearer injection, whitespace trimming, missing or empty files, and token rotation;
- absence of JWT values from HTTP errors;
- URL and HTTP status handling;
- the get_server_identity core use case;
- an end-to-end MCP stdio call against a fake OpenSVC daemon.

## Design principles

- Keep the OpenSVC daemon API client generic and internal.
- Keep endpoint selection and OpenSVC semantics in the core layer.
- Keep MCP schemas and registration in the tools layer.
- Register each tool domain explicitly in main.go.
- Prefer typed, bounded tools over arbitrary API access.
- Do not expose credentials or raw secrets to MCP clients or language models.
- Add authentication and policy enforcement before state-changing tools.
- Verify OpenSVC operations after execution instead of assuming request acceptance means completion.

## Roadmap

Near-term work is expected to focus on:

1. Basic Auth and X.509 client-certificate authentication;
2. trusted TLS and custom CA configuration;
3. Unix socket support for local development;
4. richer tests against representative OpenSVC v3 responses;
5. stable error contracts;
6. additional read-only tools driven by operational use cases;
7. HTTP MCP transport and caller authentication;
8. audited, policy-controlled state-changing tools.

## License

See the LICENSE file.

## Project Status

This project is currently in development. Feedback, issues, and contributions are welcome.

For questions or discussion, you can contact me on LinkedIn:

https://fr.linkedin.com/in/hugo-brenet-49b200202
