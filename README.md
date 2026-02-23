# warp-go-server

A lightweight HTTP tunneling server written in Go. It allows remote clients to expose local services over the internet by tunneling HTTP requests through a persistent WebSocket connection — similar in concept to tools like ngrok or Cloudflare Tunnel.

## How It Works

```
Internet → warp-go-server → WebSocket tunnel → Local client → Local service
```

1. A client connects to the server via WebSocket at `/_connect`
2. The client sends a `register` message to claim a domain
3. Incoming HTTP requests for that domain are forwarded to the client over the WebSocket
4. The client processes the request locally and streams the response back through the tunnel

## Requirements

- Go 1.22.3 or later

## Installation

```bash
git clone https://github.com/mcandeia/warp-go-server
cd warp-go-server
go build -o warp-go-server .
```

## Usage

Start the server:

```bash
./warp-go-server
```

By default it listens on port `8001`. Use the `-port` flag to change this:

```bash
./warp-go-server -port 9000
```

## Endpoints

| Endpoint | Description |
|---|---|
| `/_connect` | WebSocket endpoint for tunnel clients to connect |
| `/_healthcheck` | Health check — returns `OK` when the server is healthy |
| `/*` | All other requests are routed to the registered tunnel client for the request's `Host` header |

## WebSocket Protocol

Once a client connects to `/_connect`, it communicates with the server using a JSON message protocol.

### Client → Server Messages

**`register`** — claim a domain to receive traffic for:
```json
{
  "type": "register",
  "id": "<message-id>",
  "apiKey": "<your-api-key>",
  "domain": "myapp.example.com"
}
```

**`response-start`** — begin sending an HTTP response:
```json
{
  "type": "response-start",
  "id": "<request-id>",
  "statusCode": 200,
  "statusMessage": "OK",
  "headers": { "Content-Type": "text/html" }
}
```

**`data`** — send a chunk of response body:
```json
{
  "type": "data",
  "id": "<request-id>",
  "chunk": "<base64-encoded-bytes>"
}
```

**`data-end`** — signal the end of the response body:
```json
{
  "type": "data-end",
  "id": "<request-id>",
  "error": null
}
```

### Server → Client Messages

**`registered`** — confirms a domain registration:
```json
{
  "type": "registered",
  "id": "<message-id>",
  "domain": "myapp.example.com"
}
```

**`request-start`** — a new incoming HTTP request:
```json
{
  "type": "request-start",
  "id": "<request-id>",
  "domain": "myapp.example.com",
  "method": "GET",
  "url": "/path?query=value",
  "headers": { "Accept": "text/html" },
  "hasBody": false
}
```

**`request-data`** — a chunk of the request body:
```json
{
  "type": "request-data",
  "id": "<request-id>"
}
```
> Note: body bytes are sent as a binary WebSocket frame alongside the JSON envelope.

**`request-end`** — signals the end of the request body:
```json
{
  "type": "request-end",
  "id": "<request-id>"
}
```

## Features

- **WebSocket tunneling** — persistent, low-overhead connections using [gorilla/websocket](https://github.com/gorilla/websocket)
- **Chunked streaming** — request and response bodies are streamed in chunks
- **Request tracing** — every request gets an `X-Request-Id` header (passed through or auto-generated)
- **Graceful shutdown** — in-flight requests complete before the server exits on `SIGTERM`/`SIGINT`
- **Concurrent clients** — multiple tunnel clients can connect simultaneously, each serving their own domain

## Running Tests

```bash
go test ./...
```

## Dependencies

- [gorilla/websocket](https://github.com/gorilla/websocket) v1.5.1
- [google/uuid](https://github.com/google/uuid) v1.6.0

## License

See [LICENSE](LICENSE) for details.
