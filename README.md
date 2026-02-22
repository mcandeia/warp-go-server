# Welcome to warp-go-server

[![Go Version](https://img.shields.io/badge/go-1.22+-00ADD8?logo=go)](https://golang.org/)
[![License](https://img.shields.io/github/license/mcandeia/warp-go-server)](LICENSE)
[![GitHub Stars](https://img.shields.io/github/stars/mcandeia/warp-go-server?style=social)](https://github.com/mcandeia/warp-go-server)

> Expose your local services to the internet — no firewall rules, no static IPs, just WebSockets.

`warp-go-server` is a lightweight, self-hosted reverse tunnel written in Go. Run it on any public server, connect your local service over WebSocket, and that service becomes reachable from the internet — similar to [ngrok](https://ngrok.com/) or [Cloudflare Tunnel](https://www.cloudflare.com/products/tunnel/), but fully under your own control.

Whether you're demoing a project, testing a webhook locally, or sharing a dev environment with a teammate, `warp-go-server` has you covered. Set it up in seconds and start tunneling!

## How It Works

```mermaid
sequenceDiagram
    participant C as Tunnel Client<br/>(your local service)
    participant S as warp-go-server
    participant E as External HTTP Client

    Note over C,S: Registration Phase
    C->>S: WebSocket connect (/_connect)
    C->>S: register {domain: "myapp.example.com"}
    S-->>C: registered {domain: "myapp.example.com"}

    Note over C,S,E: Request Forwarding Phase
    E->>S: HTTP request for myapp.example.com
    S->>C: request-start {method, url, headers}
    S->>C: request-data {body chunk...}
    S->>C: request-end
    C->>S: response-start {status, headers}
    C->>S: data {body chunk...}
    C->>S: data-end
    S-->>E: HTTP response
```

1. Your local service connects to `warp-go-server` via WebSocket (`/_connect`)
2. It registers a domain with the server using a `register` message
3. Incoming HTTP requests for that domain are forwarded through the WebSocket tunnel to your local service
4. Your service handles the request and streams the response back through the same tunnel

No inbound ports. No firewall changes. Just connect and go.

## Features

- **WebSocket-based persistent tunnel** — one long-lived connection handles all traffic
- **Streaming support** — request and response bodies are chunked for efficient transfer
- **Multiple clients** — run many tunnel clients simultaneously, each with their own domain
- **Request tracing** — every request gets an `X-Request-Id` header for easy debugging
- **Health check endpoint** — quickly verify the server is up and ready
- **Graceful shutdown** — handles `SIGINT`/`SIGTERM` cleanly without dropping active connections

## Getting Started

### Prerequisites

- Go 1.22+

### Build

```bash
go build -o warp-go-server .
```

### Run

```bash
./warp-go-server -port 8001
```

The server starts on port `8001` by default. Once it's running, your tunnel clients can connect and start registering domains.

## API

### `/_healthcheck`

Returns `OK` (HTTP 200) when the server is healthy, or HTTP 503 when it's shutting down. Use this for load balancer health probes or uptime monitoring.

### `/_connect` (WebSocket)

The tunnel endpoint. Clients connect here, register their domain, and the server begins forwarding matching HTTP requests to them.

**Client → Server messages:**

| Type | Description |
|------|-------------|
| `register` | Register a domain to proxy traffic to this client |
| `response-start` | Send HTTP response status and headers for an ongoing request |
| `data` | Send a chunk of the HTTP response body |
| `data-end` | Signal end of the HTTP response body |
| `ws-opened` | Notify that a WebSocket connection was opened on the client side |
| `ws-message` | Forward a WebSocket message from the client |
| `ws-closed` | Notify that a WebSocket connection was closed on the client side |

**Server → Client messages:**

| Type | Description |
|------|-------------|
| `registered` | Acknowledgement that the domain was registered successfully |
| `request-start` | Notify client of an incoming HTTP request |
| `request-data` | Send a chunk of the HTTP request body |
| `request-end` | Signal end of the HTTP request body |
| `error` | Report an error back to the client |

### Registration flow

```json
// Client sends:
{
  "type": "register",
  "id": "<uuid>",
  "apiKey": "<your-api-key>",
  "domain": "myapp.example.com"
}

// Server responds:
{
  "type": "registered",
  "id": "<uuid>",
  "domain": "myapp.example.com"
}
```

### Request forwarding flow

Once registered, the server automatically forwards incoming requests:

```
1. External HTTP request arrives at warp-go-server for "myapp.example.com"
2. Server sends "request-start" to the registered client
3. Server streams request body via "request-data" messages
4. Server signals "request-end"
5. Client sends "response-start" with status code and headers
6. Client streams response body via "data" messages
7. Client sends "data-end" to complete the response
```

## Project Structure

Current layout — everything lives in a single flat package:

```
.
├── main.go               # Entry point, HTTP server setup, middleware
└── pkg/
    └── server/
        ├── server.go              # Core server logic and HTTP/WS handlers
        ├── messages.go            # Message type definitions and handlers
        ├── messages_serializer.go # WebSocket message serializer
        ├── ws.go                  # Generic duplex WebSocket channel
        ├── json_serializer.go     # JSON serializer implementation
        ├── arraybuffer_serializer.go # Binary (arraybuffer) serializer
        ├── writable_stream.go     # Writable stream abstraction
        └── server_test.go         # Tests
```

### Proposed reorganization

**Approach 1 — Split by concern (protocol vs. transport vs. proxy)**

Group files around what they do rather than where they live:

```
.
├── main.go
└── pkg/
    ├── protocol/          # Message types and serialization
    │   ├── messages.go
    │   ├── messages_serializer.go
    │   ├── json_serializer.go
    │   └── arraybuffer_serializer.go
    ├── tunnel/            # WebSocket transport layer
    │   ├── ws.go
    │   └── writable_stream.go
    └── proxy/             # HTTP reverse-proxy and routing
        ├── server.go
        └── server_test.go
```

This makes `protocol`, `tunnel`, and `proxy` independently importable and testable — useful if a client library later reuses the protocol or transport layer.

---

**Approach 2 — Flatten into `internal/` with logical file grouping**

Keep a single package but move it under `internal/` (preventing accidental external imports) and rename files so their purpose is self-evident:

```
.
├── main.go
└── internal/
    ├── handler.go         # HTTP and WebSocket request handlers (was server.go)
    ├── message.go         # Message type definitions (was messages.go)
    ├── serializer.go      # All serializer implementations merged (was *_serializer.go)
    ├── stream.go          # WebSocket channel + writable stream (was ws.go + writable_stream.go)
    └── handler_test.go    # Tests
```

This reduces the file count from 8 to 5, eliminates the redundant `pkg/server/` nesting, and groups the two serializer files that always change together.

## Configuration

| Flag | Default | Description |
|------|---------|-------------|
| `-port` | `8001` | Port the server listens on |

## Running Tests

```bash
go test ./...
```

## Dependencies

- [gorilla/websocket](https://github.com/gorilla/websocket) — WebSocket implementation
- [google/uuid](https://github.com/google/uuid) — UUID generation for request/client IDs

## Contributing

Issues and pull requests are welcome! Feel free to open an issue if you run into a bug or have an idea for an improvement.

## License

See [LICENSE](LICENSE) for details.
