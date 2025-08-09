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
- **`internal/server`**: Handles HTTP/SSE transport and session management.
- **`internal/knowledge`**: The core business logic, mapping MCP tools to storage operations.
- **`internal/storage`**: The storage layer, with a `Backend` interface and a concrete `SqliteBackend` implementation.
- **`pkg/config`**: Handles loading configuration from YAML files.
- **`pkg/mcp`**: Contains the core MCP data type definitions.

## ⚙️ Configuration

Configuration is managed via a `config.yaml` file. Create one in the root of the project:

```yaml
# config.yaml

# Transport type: stdio, http, or sse
transportType: "stdio"

# Port for HTTP/SSE transport
httpPort: 8080

# Storage backend type (currently only sqlite is supported)
storageType: "sqlite"

# Path to the SQLite database file
storagePath: "./knowledge.db"

# Log level: debug, info, warn, error
logLevel: "info"

# SQLite specific settings
sqlite:
  walMode: true # Write-Ahead Logging is recommended for concurrent access
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
