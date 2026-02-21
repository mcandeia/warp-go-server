# warp-go-server

A lightweight reverse tunnel server written in Go that forwards HTTP requests to clients over WebSocket connections — similar to [ngrok](https://ngrok.com/) or [Cloudflare Tunnel](https://www.cloudflare.com/products/tunnel/).

## How It Works

```
Internet ──► warp-go-server ──► WebSocket ──► Your local service
```

1. Your local service connects to `warp-go-server` via WebSocket (`/_connect`)
2. It registers a domain with the server using a `register` message
3. Incoming HTTP requests for that domain are forwarded through the WebSocket tunnel to your local service
4. Your service handles the request and streams the response back through the same tunnel

This allows you to expose a locally running HTTP service to the public internet without opening any inbound ports.

## Features

- WebSocket-based persistent tunnel
- Streaming request and response bodies (chunked transfer)
- Multiple concurrent tunnel clients, each with their own domain
- Request tracing via `X-Request-Id` headers
- Health check endpoint
- Graceful shutdown on `SIGINT`/`SIGTERM`

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

By default the server listens on port `8001`.

## API

### `/_healthcheck`

Returns `OK` (HTTP 200) when the server is healthy, or HTTP 503 when shutting down.

### `/_connect` (WebSocket)

Tunnel endpoint. Clients connect here and register their domain.

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
| `registered` | Acknowledgement that the domain was registered |
| `request-start` | Notify client of an incoming HTTP request |
| `request-data` | Send a chunk of the HTTP request body |
| `request-end` | Signal end of the HTTP request body |
| `error` | Report an error |

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

## License

See [LICENSE](LICENSE) for details.
