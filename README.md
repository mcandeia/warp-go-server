# warp-go-server

A reverse proxy tunneling server written in Go that forwards HTTP requests to remote clients over WebSocket connections. It enables exposing local services to the internet without direct inbound connectivity.

## Overview

`warp-go-server` acts as a relay between external HTTP clients and local services. A remote client connects to the server via WebSocket, registers the domain(s) it wants to handle, and the server forwards all matching HTTP requests to that client. The client processes each request locally and streams the response back through the WebSocket tunnel.

```
External Request → warp-go-server → WebSocket Tunnel → Local Service
```

## Architecture

```mermaid
graph TB
    subgraph Internet
        A[HTTP Client]
    end

    subgraph warp-go-server
        B[HTTP Server :8001]
        C[Tracing Middleware]
        D[Logging Middleware]
        E[/_connect\nWebSocket Handler]
        F[/*\nRequest Handler]
        G[/_healthcheck\nHealth Check]
        H[(hostToClientID\nSync Map)]
        I[(serverStates\nSync Map)]
    end

    subgraph Remote Client
        J[WebSocket Client]
        K[Local Service\ne.g. localhost:3000]
    end

    A -->|HTTP Request| B
    B --> C --> D
    D --> E
    D --> F
    D --> G
    E -->|Upgrade to WS| J
    J -->|RegisterMessage| H
    F -->|Lookup host| H
    H -->|clientID| I
    I -->|DuplexChan| J
    J <-->|Bidirectional\nMessage Stream| K
```

## Request Flow

```mermaid
sequenceDiagram
    participant EC as External Client
    participant WS as warp-go-server
    participant RC as Remote Client (via WS)
    participant LS as Local Service

    Note over RC,WS: Connection Setup
    RC->>WS: WebSocket connect (/_connect)
    RC->>WS: RegisterMessage {domain: "example.com"}
    WS-->>RC: RegisteredMessage {domain: "example.com"}

    Note over EC,LS: HTTP Request Forwarding
    EC->>WS: GET http://example.com/api/data
    WS->>RC: RequestStartMessage {method, url, headers}
    WS->>RC: RequestDataEndMessage
    RC->>LS: Forward request locally
    LS-->>RC: HTTP Response

    RC->>WS: ResponseStartMessage {statusCode, headers}
    RC->>WS: DataMessage {chunk}
    RC->>WS: DataMessage {chunk}
    RC->>WS: DataEndMessage
    WS-->>EC: HTTP Response (streamed)
```

## Message Protocol

```mermaid
graph LR
    subgraph Client to Server Messages
        A[register] --> A1["Register domain\n{id, type, apiKey, domain}"]
        B[response-start] --> B1["Send HTTP response headers\n{id, statusCode, headers}"]
        C[data] --> C1["Send response body chunk\n{id, chunk: binary}"]
        D[data-end] --> D1["Signal end of response\n{id, error}"]
        E[ws-opened] --> E1["WebSocket connection opened"]
        F[ws-message] --> F1["WebSocket message\n{id, data}"]
        G[ws-closed] --> G1["WebSocket connection closed"]
    end

    subgraph Server to Client Messages
        H[request-start] --> H1["Initial request metadata\n{id, domain, method, url, headers, hasBody}"]
        I[request-data] --> I1["Request body chunk\n{id, chunk: binary}"]
        J[request-end] --> J1["End of request body\n{id}"]
        K[registered] --> K1["Acknowledge domain registration\n{id, domain}"]
        L[error] --> L1["Error message\n{message}"]
    end
```

## Wire Format

Messages use a custom binary protocol that combines JSON metadata with an optional binary payload:

```
┌─────────────────────────────────────────────────────────┐
│  4 bytes (LE)  │  N bytes (JSON)  │  M bytes (binary)   │
│  metadata len  │  message metadata│  binary payload     │
└─────────────────────────────────────────────────────────┘
```

This allows efficient streaming of binary data (e.g., response bodies) without base64 encoding.

## Getting Started

### Prerequisites

- Go 1.22.3+

### Build

```bash
git clone https://github.com/mcandeia/warp-go-server.git
cd warp-go-server
go build -o warp-go-server .
```

### Run

```bash
./warp-go-server
# Server listens on :8001 by default

./warp-go-server -port 9000
# Listen on a custom port
```

## API Endpoints

| Endpoint | Method | Description |
|---|---|---|
| `/_connect` | `GET` (WebSocket) | Client connection endpoint — upgrades to WebSocket |
| `/_healthcheck` | `GET` | Returns `200 OK` when healthy, `503` during shutdown |
| `/*` | `*` | Catch-all — proxies all other requests to the registered client |

## Configuration

| Flag | Default | Description |
|---|---|---|
| `-port` | `8001` | Port the server listens on |

## Project Structure

```
warp-go-server/
├── main.go                          # Entry point, middleware setup, graceful shutdown
└── pkg/server/
    ├── server.go                    # Core server: WebSocket handler and request proxy
    ├── messages.go                  # Message type definitions and handlers
    ├── messages_serializer.go       # Custom binary+JSON serializer
    ├── arraybuffer_serializer.go    # Binary wire format implementation
    ├── json_serializer.go           # Generic JSON serializer
    ├── ws.go                        # Generic bidirectional WebSocket channel
    ├── writable_stream.go           # Thread-safe streaming buffer
    └── server_test.go               # Tests
```

## Concurrency Model

```mermaid
graph TB
    subgraph Per WebSocket Connection
        A[onWSConnect goroutine] -->|select loop| B[Message Dispatcher]
        B --> C[RegisterMessage Handler]
        B --> D[ResponseStartMessage Handler]
        B --> E[DataMessage Handler]
        B --> F[DataEndMessage Handler]
    end

    subgraph Per HTTP Request
        G[onRequest goroutine] -->|WaitGroup| H[Response Writer goroutine]
        H -->|reads from| I[respBodyChan]
        E -->|writes to| I
        F -->|closes| I
    end

    subgraph DuplexChan
        J[Send goroutine] -->|serialize + write| K[WebSocket conn]
        K -->|read + deserialize| L[Receive goroutine]
        L -->|push to| M[recv channel]
        B -->|reads from| M
    end
```

## Graceful Shutdown

On `SIGTERM` or `SIGINT`:
1. The server stops accepting new connections
2. The health check endpoint returns `503 Service Unavailable`
3. Existing connections are given 30 seconds to complete
4. The server exits cleanly

## Dependencies

| Package | Purpose |
|---|---|
| `github.com/gorilla/websocket` | WebSocket implementation |
| `github.com/google/uuid` | Unique IDs for clients and requests |
