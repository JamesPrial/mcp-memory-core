# Phase 4: Transport Layer Error Mapping Implementation

## Overview
This phase implements comprehensive error mapping for all transport layers (HTTP, SSE, STDIO) with proper JSON-RPC error codes and HTTP status code mapping.

## Components Implemented

### 1. MCP Error Type (`pkg/mcp/types.go`)
- Added `Error` struct compatible with JSON-RPC error format
- Provides consistent error structure across all transport layers

### 2. Error Mapper (`internal/transport/error_mapper.go`)
- **`ToJSONRPCError(err error) mcp.Error`**: Maps internal AppError codes to JSON-RPC error codes
- **`ToJSONRPCResponse(id, err) *JSONRPCResponse`**: Creates complete JSON-RPC responses
- **`ToHTTPStatusCode(err error) int`**: Maps errors to appropriate HTTP status codes
- **`CreateFallbackErrorResponse(id, message) *JSONRPCResponse`**: Safe fallback for critical failures
- **`SafeErrorMessage(err error) string`**: Client-safe error messages
- **`LoggableError(err error) error`**: Full error details for logging

### 3. JSON-RPC Error Code Mapping
Following JSON-RPC 2.0 specification:
- **-32700**: Parse error (transport-level JSON parsing)
- **-32600**: Invalid Request 
- **-32601**: Method not found
- **-32602**: Invalid params (validation errors)
- **-32603**: Internal error (system errors)
- **-32000 to -32099**: Custom application errors
  - -32001: Not Found errors
  - -32002: Already Exists errors
  - -32003: Conflict errors
  - -32004: Unauthorized errors
  - -32005: Forbidden errors

### 4. HTTP Status Code Mapping
Following JSON-RPC over HTTP best practices:
- **Transport-level errors**: Return appropriate HTTP error codes (400, 408, 429, 503, 500)
- **Application-level errors**: Return HTTP 200 with error details in JSON-RPC response body
- **Parse errors**: HTTP 400 for malformed requests, HTTP 200 for JSON-RPC parse errors

### 5. Updated Transport Implementations

#### HTTP Transport (`internal/transport/http.go`)
- Uses error mapper for all error responses
- Proper HTTP status codes based on error types
- Fallback error handling for critical failures
- Maintains JSON-RPC compliance (parse errors return HTTP 200)

#### SSE Transport (`internal/transport/sse.go`)
- Consistent error handling across SSE events and HTTP endpoints
- Error events sent via SSE for connection-level errors
- Proper error responses for HTTP POST endpoints

#### STDIO Transport (`internal/transport/stdio.go`)
- Uses error mapper for all responses
- Multiple fallback levels for critical serialization failures
- Maintains consistent error format

### 6. Error Recovery Mechanisms
- **Fallback responses**: Always return valid JSON-RPC error even on critical failures
- **Multiple fallback levels**: If primary error serialization fails, use simpler fallback
- **Safe message handling**: Never leak internal details to clients
- **Logging support**: Full error context available for server-side logging

## Key Features

1. **Security**: Internal error details never leaked to clients
2. **Reliability**: Always returns valid JSON-RPC responses even on critical failures
3. **Consistency**: Same error format across all transport layers
4. **Compliance**: Follows JSON-RPC 2.0 and HTTP best practices
5. **Observability**: Rich logging context while maintaining client safety

## Testing
- Comprehensive test suite covering all error mapper functions
- All existing transport tests still pass
- Error scenarios properly handled and tested

## Usage Examples

```go
// Map internal error to JSON-RPC response
err := errors.New(errors.ErrCodeEntityNotFound, "User not found")
response := ToJSONRPCResponse("req-123", err)
statusCode := ToHTTPStatusCode(err) // Returns 200 for application errors

// Create fallback response for critical failures
fallback := CreateFallbackErrorResponse("req-123", "System error")

// Get safe message for client
safeMsg := SafeErrorMessage(err) // "User not found"

// Get full details for logging
logErr := LoggableError(err) // Full context including error code
```

This implementation ensures that all transport layers handle errors consistently while maintaining security and following established protocols.