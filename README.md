# OpenSVC Daemon MCP

A Go-based Model Context Protocol server that gives AI agents a controlled, typed interface to the OpenSVC v3 daemon API.

The project is intended to become the low-level operational MCP layer for AI-assisted inspection, diagnosis, and administration of OpenSVC clusters. One MCP server is expected to run close to each OpenSVC daemon and expose carefully designed tools instead of a generic raw API proxy.

## Project status

This project is in an early development stage.

The current implementation:

- runs as a Streamable HTTP MCP server on a loopback address;
- uses the official Go MCP SDK;
- connects to a configurable OpenSVC daemon API URL;
- requires an OpenSVC Bearer access JWT on every MCP request;
- validates JWT signatures and claims before invoking the MCP handler;
- delegates the same request-scoped JWT to the daemon API;
- exposes a small, typed diagnostic tool surface with explicit safety annotations;
- returns filtered, bounded structured responses;
- preserves bounded RFC 7807 daemon error details in MCP tool errors;
- supports a custom CA bundle for daemon server verification.

It is not production-ready.

## Tool documentation

The MCP currently exposes mostly read-only tools plus one explicit,
non-destructive instance status refresh.
Detailed documentation is organized by OpenSVC daemon domain and includes
verified, normalized lab input/output examples:

- [Tool index and diagnostic workflow](docs/tools/README.md)
- [Daemon tools](docs/tools/daemon.md)
- [Cluster tools](docs/tools/cluster.md)
- [Object tools](docs/tools/objects.md)
- [Instance tools](docs/tools/instances.md)
- [Resource tools](docs/tools/resources.md)

## Requirements

- Go 1.25.5 or later
- Access to an OpenSVC v3 daemon API
- The public certificate or RSA public key of the OpenSVC cluster CA
- An OpenSVC access JWT for the MCP client
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
| OPENSVC_DAEMON_REQUEST_TIMEOUT | 20s | Whole-request timeout for daemon JSON, SSE, and bounded stream calls; accepted range 1s to 2m |
| OPENSVC_MCP_LISTEN_ADDRESS | 127.0.0.1:8080 | Streamable HTTP listen address; currently restricted to a loopback IP |
| OPENSVC_MCP_JWT_VERIFY_KEY_FILE | /var/lib/opensvc/certs/ca_certificates | OpenSVC cluster CA certificate or RSA public key used to verify JWT signatures |
| OPENSVC_DAEMON_TLS_CA_FILE | empty | PEM CA certificates appended to the system trust store |
| OPENSVC_DAEMON_TLS_INSECURE | false | Disable daemon certificate verification. Development only. |

Example:

~~~bash
export OPENSVC_DAEMON_URL=https://127.0.0.1:1215
export OPENSVC_MCP_LISTEN_ADDRESS=127.0.0.1:8080
export OPENSVC_MCP_JWT_VERIFY_KEY_FILE=/var/lib/opensvc/certs/ca_certificates
~~~

For a local development daemon using a self-signed certificate, verification can be explicitly disabled:

~~~bash
export OPENSVC_DAEMON_TLS_INSECURE=true
~~~

This disables certificate-chain and hostname verification. Never enable it when connecting to a daemon over an untrusted network. The default remains secure.

The configured verification file contains public material only, but it must be readable by the MCP process. Never expose or mount `/var/lib/opensvc/certs/ca_private_key` into the MCP server.

Each MCP HTTP request must contain:

~~~text
Authorization: Bearer <jwt>
~~~

The middleware accepts only JWTs signed with RS256 by the configured cluster CA. It requires valid `exp`, `sub`, `iss`, and `token_use=access` claims. The authenticated subject is bound to the MCP session to prevent session hijacking. The raw JWT remains request-scoped and is forwarded to the daemon, which independently validates it and applies its `grant` claims.

There is no Basic Auth, X.509 client-authentication, local token file, unauthenticated mode, or fallback service credential.

## Run

Start the Streamable HTTP MCP server:

~~~bash
OPENSVC_DAEMON_URL=https://127.0.0.1:1215 \
OPENSVC_MCP_LISTEN_ADDRESS=127.0.0.1:8080 \
OPENSVC_MCP_JWT_VERIFY_KEY_FILE=/var/lib/opensvc/certs/ca_certificates \
OPENSVC_DAEMON_TLS_INSECURE=true \
  ./bin/opensvc-daemon-mcp
~~~

The MCP endpoint is `http://127.0.0.1:8080/mcp`. Until server-side TLS is implemented, configuration rejects non-loopback listen addresses so Bearer tokens cannot cross an unencrypted network. `OPENSVC_DAEMON_TLS_INSECURE` affects only the separate MCP-to-daemon HTTPS connection.

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
- bounded finite SSE reads for OpenSVC instance logs;
- bounded opaque stream reads for container stdout and stderr logs;
- RS256 JWT verification and required OpenSVC access-token claims;
- rejection of missing, invalid, expired, and refresh Bearer tokens;
- request-scoped Bearer delegation to the daemon;
- custom server CA loading and TLS verification;
- absence of JWT values from HTTP errors;
- URL and HTTP status handling;
- bounded RFC 7807 error propagation through real MCP tool calls, including malformed and interrupted responses;
- the current core use cases and their bounded response shaping;
- end-to-end Streamable HTTP MCP calls to every registered tool using a delegated JWT against a fake OpenSVC daemon.

## Design principles

- Keep the OpenSVC daemon API client generic and internal.
- Keep endpoint selection and OpenSVC semantics in the core layer.
- Keep MCP schemas and registration in the tools layer.
- Keep user-facing tool documentation in docs/tools.
- Register each tool domain explicitly in main.go.
- Prefer typed, bounded tools over arbitrary API access.
- Do not expose credentials or raw secrets to MCP clients or language models.
- Add authentication and policy enforcement before state-changing tools.
- Verify OpenSVC operations after execution instead of assuming request acceptance means completion.
- Treat status returned by read-only GET tools as the daemon's last-known state; these tools do not implicitly run resource-driver probes.

## Roadmap

Near-term work is expected to focus on:

1. TLS for the Streamable HTTP MCP server and controlled non-loopback binding;
2. richer tests against representative OpenSVC v3 responses;
3. stable error and audit contracts;
4. additional read-only tools driven by operational use cases;
5. audited, policy-controlled state-changing tools.

## License

See the LICENSE file.

## Project Status

This project is currently in development. Feedback, issues, and contributions are welcome.

For questions or discussion, you can contact me on LinkedIn:

https://fr.linkedin.com/in/hugo-brenet-49b200202
