# MCP Memory Core

[![Go Report Card](https://goreportcard.com/badge/github.com/JamesPrial/mcp-memory-core)](https://goreportcard.com/report/github.com/JamesPrial/mcp-memory-core)
[![Go.Dev reference](https://img.shields.io/badge/go.dev-reference-blue?logo=go&logoColor=white)](https://pkg.go.dev/github.com/JamesPrial/mcp-memory-core)

**`mcp-memory-core`** is a high-performance, production-grade implementation of the Model-Context Protocol (MCP) memory server, completely rewritten in Go. It is designed from the ground up for performance, stability, and type-safety, serving as a robust replacement for the original TypeScript server.

This project was developed using a rigorous Test-Driven Development (TDD) methodology to ensure reliability and correctness.

## 🚀 Features

- **High Performance:** Built in Go for significant speed improvements and lower memory usage compared to the original implementation.
- **Enhanced Stability:** A strong focus on stability and concurrency-safety makes it suitable for production workloads.
- **Type-Safe:** Leverages Go's static typing to eliminate a whole class of runtime errors.
- **SQLite Backend:** Uses a powerful SQLite backend with WAL-mode for efficient and concurrent data access.
- **Multiple Transport Layers:** Supports standard `stdio` for traditional MCP clients as well as `http` and `sse` for modern, web-based integrations.
- **Production Ready:** Includes Docker support, health checks, and a clean, modular architecture.

## 🔧 Architecture

The server is designed with a clean separation of concerns:

```
┌───────────────────┐
│   MCP Client      │ (stdio, http, sse)
└─────────┬─────────┘
          │
┌─────────▼─────────┐
│ Transport Layer   │ (Handles JSON-RPC)
└─────────┬─────────┘
          │
┌─────────▼─────────┐
│ Knowledge Manager │ (Core Business Logic)
└─────────┬─────────┘
          │
┌─────────▼─────────┐
│  Storage Backend  │ (Interface)
└─────────┬─────────┘
          │
┌─────────▼─────────┐
│  SQLite Provider  │ (Implementation)
└───────────────────┘
```

- **`cmd/mcp-server`**: The main application entrypoint.
- **`internal/transport`**: Transport layer with support for stdio, HTTP, and SSE protocols.
- **`internal/knowledge`**: The core business logic, mapping MCP tools to storage operations.
- **`internal/storage`**: The storage layer, with a `Backend` interface and concrete implementations.
- **`pkg/config`**: Handles loading configuration from YAML files.
- **`pkg/mcp`**: Contains the core MCP data type definitions.

## ⚙️ Configuration

Configuration is managed via a `config.yaml` file. Create one in the root of the project:

### Basic Configuration (stdio transport)

```yaml
# config.yaml - Standard stdio transport for MCP clients

# Transport type: stdio (default), http, or sse
transportType: "stdio"

# Storage backend type: memory or sqlite
storageType: "sqlite"

# Path to the SQLite database file
storagePath: "./knowledge.db"

# Log level: debug, info, warn, error
logLevel: "info"

# SQLite specific settings
sqlite:
  walMode: true # Write-Ahead Logging for better concurrency
```

### HTTP Transport Configuration

```yaml
# config.http.yaml - HTTP transport for synchronous communication

transportType: "http"
storageType: "memory"

# HTTP transport settings
transport:
  host: "localhost"       # Host to bind to
  port: 8080             # Port to listen on
  readTimeout: 30        # Read timeout in seconds
  writeTimeout: 30       # Write timeout in seconds
  maxConnections: 100    # Maximum concurrent connections
  enableCors: true       # Enable CORS for browser clients

logLevel: "info"
```

### SSE Transport Configuration

```yaml
# config.sse.yaml - Server-Sent Events for real-time communication

transportType: "sse"
storageType: "sqlite"
storagePath: "./memory.db"

# SSE transport settings
transport:
  host: "0.0.0.0"          # Bind to all interfaces
  port: 8080               # Port to listen on
  readTimeout: 30          # Read timeout in seconds
  writeTimeout: 30         # Write timeout in seconds
  maxConnections: 100      # Maximum concurrent connections
  enableCors: true         # Enable CORS for browser clients
  sseHeartbeatSecs: 30     # Heartbeat interval (keeps connections alive)

sqlite:
  walMode: true

logLevel: "debug"
```

## 🚀 Getting Started

### Prerequisites

- Go 1.18 or later

### Build and Run

1.  **Clone the repository:**
    ```bash
    git clone https://github.com/JamesPrial/mcp-memory-core.git
    cd mcp-memory-core
    ```

2.  **Install dependencies:**
    ```bash
    go mod tidy
    ```

3.  **Build the binary:**
    ```bash
    go build -o mcp-memory-core ./cmd/mcp-server
    ```

4.  **Run the server:**
    ```bash
    ./mcp-memory-core --config config.yaml
    ```

## 🔌 Transport Modes

The server supports three transport modes for different use cases:

### stdio (Standard I/O)
- **Use Case**: Traditional MCP clients, command-line tools
- **Protocol**: JSON-RPC over stdin/stdout
- **Example**: `./mcp-memory-core --config config.yaml`

### HTTP
- **Use Case**: Web applications, REST-like APIs, synchronous communication
- **Protocol**: JSON-RPC over HTTP POST
- **Endpoints**:
  - `POST /rpc` - Send JSON-RPC requests
  - `GET /health` - Health check endpoint
- **Features**: Session management, CORS support, request/response pattern
- **Example Usage**:
  ```bash
  curl -X POST http://localhost:8080/rpc \
    -H "Content-Type: application/json" \
    -H "X-Session-ID: your-session-id" \
    -d '{"jsonrpc":"2.0","id":1,"method":"tools/list","params":{}}'
  ```

### SSE (Server-Sent Events)
- **Use Case**: Real-time web applications, streaming updates, browser-based clients
- **Protocol**: JSON-RPC with SSE for server-to-client streaming
- **Endpoints**:
  - `GET /events` - Establish SSE connection
  - `POST /rpc` - Send JSON-RPC requests (responses via SSE)
  - `GET /health` - Health check with client count
- **Features**: Real-time bidirectional communication, automatic reconnection, heartbeat
- **Example Usage**:
  ```javascript
  // Connect to SSE
  const eventSource = new EventSource('http://localhost:8080/events');
  let sessionId = null;
  
  eventSource.addEventListener('session', (e) => {
    const data = JSON.parse(e.data);
    sessionId = data.sessionId;
  });
  
  eventSource.addEventListener('message', (e) => {
    const response = JSON.parse(e.data);
    console.log('Received:', response);
  });
  
  // Send request via HTTP POST
  fetch('http://localhost:8080/rpc', {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
      'X-Session-ID': sessionId
    },
    body: JSON.stringify({
      jsonrpc: '2.0',
      id: 1,
      method: 'tools/list',
      params: {}
    })
  });
  ```

## 🐳 Docker

A `Dockerfile` is provided for building and running the server in a container.

1.  **Build the image:**
    ```bash
    docker build -t mcp-memory-core .
    ```

2.  **Run the container:**
    ```bash
    docker run -d \
      --name mcp-core \
      -v $(pwd)/data:/data \
      -p 8080:8080 \
      mcp-memory-core
    ```
    *Note: You will need to create a `config.yaml` that points `storagePath` to `/data/knowledge.db` and set `transportType` to `http`.*

## 🛠️ Development & Testing

This project uses the standard Go testing framework. To run all tests, including race condition checks:

```bash
go test -race ./...
```
