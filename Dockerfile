# Build stage
FROM golang:1.22-alpine AS builder

# Install build dependencies
RUN apk add --no-cache git gcc musl-dev sqlite-dev

# Set working directory
WORKDIR /app

# Copy go mod files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy source code
COPY . .

# Build the application
RUN CGO_ENABLED=1 GOOS=linux go build -a -installsuffix cgo -o mcp-server ./cmd/mcp-server

# Final stage
FROM alpine:latest

# Install runtime dependencies
RUN apk --no-cache add ca-certificates sqlite

WORKDIR /root/

# Copy the binary from builder
COPY --from=builder /app/mcp-server .

# Copy default config
COPY --from=builder /app/config.yaml .

# Create data directory for SQLite
RUN mkdir -p /data

# Set environment variables
ENV MCP_STORAGE_PATH=/data/memory.db
ENV MCP_LOG_LEVEL=info

# Expose port if needed (MCP servers typically use stdio)
# EXPOSE 8080

# Run the application
CMD ["./mcp-server"]