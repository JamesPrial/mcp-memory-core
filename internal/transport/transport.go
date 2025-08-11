package transport

import (
	"context"
	"encoding/json"
)

// JSONRPCRequest represents a JSON-RPC 2.0 request
type JSONRPCRequest struct {
	JSONRPC string                 `json:"jsonrpc"`
	ID      interface{}            `json:"id"`
	Method  string                 `json:"method"`
	Params  map[string]interface{} `json:"params,omitempty"`
}

// JSONRPCResponse represents a JSON-RPC 2.0 response
type JSONRPCResponse struct {
	JSONRPC string        `json:"jsonrpc"`
	ID      interface{}   `json:"id"`
	Result  interface{}   `json:"result,omitempty"`
	Error   *JSONRPCError `json:"error,omitempty"`
}

// JSONRPCError represents a JSON-RPC 2.0 error
type JSONRPCError struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// RequestHandler is a function that handles JSON-RPC requests
type RequestHandler func(ctx context.Context, req *JSONRPCRequest) *JSONRPCResponse

// Transport defines the interface for different transport mechanisms
type Transport interface {
	// Start begins listening for requests
	Start(ctx context.Context, handler RequestHandler) error
	
	// Stop gracefully shuts down the transport
	Stop(ctx context.Context) error
	
	// Name returns the name of the transport
	Name() string
}

// Message wraps a JSON-RPC message with metadata
type Message struct {
	SessionID string
	Request   *JSONRPCRequest
	Response  chan *JSONRPCResponse
}

// Session represents a client session
type Session struct {
	ID           string
	Transport    string
	CreatedAt    int64
	LastActivity int64
	Metadata     map[string]interface{}
}

// Common JSON-RPC error codes
const (
	ParseError     = -32700
	InvalidRequest = -32600
	MethodNotFound = -32601
	InvalidParams  = -32602
	InternalError  = -32603
)

// NewParseError creates a parse error response
func NewParseError() *JSONRPCResponse {
	return &JSONRPCResponse{
		JSONRPC: "2.0",
		ID:      nil,
		Error: &JSONRPCError{
			Code:    ParseError,
			Message: "Parse error",
			Data:    "Invalid JSON format",
		},
	}
}

// NewInvalidRequestError creates an invalid request error response
func NewInvalidRequestError(id interface{}, data string) *JSONRPCResponse {
	return &JSONRPCResponse{
		JSONRPC: "2.0",
		ID:      id,
		Error: &JSONRPCError{
			Code:    InvalidRequest,
			Message: "Invalid Request",
			Data:    data,
		},
	}
}

// NewMethodNotFoundError creates a method not found error response
func NewMethodNotFoundError(id interface{}, method string) *JSONRPCResponse {
	return &JSONRPCResponse{
		JSONRPC: "2.0",
		ID:      id,
		Error: &JSONRPCError{
			Code:    MethodNotFound,
			Message: "Method not found",
			Data:    "Method '" + method + "' is not supported",
		},
	}
}

// NewInternalError creates an internal error response
func NewInternalError(id interface{}, message string) *JSONRPCResponse {
	return &JSONRPCResponse{
		JSONRPC: "2.0",
		ID:      id,
		Error: &JSONRPCError{
			Code:    InternalError,
			Message: "Internal error",
			Data:    message,
		},
	}
}

// ParseRequest parses a JSON byte array into a JSONRPCRequest
func ParseRequest(data []byte) (*JSONRPCRequest, error) {
	var req JSONRPCRequest
	err := json.Unmarshal(data, &req)
	return &req, err
}

// SerializeResponse converts a JSONRPCResponse to JSON bytes
func SerializeResponse(resp *JSONRPCResponse) ([]byte, error) {
	return json.Marshal(resp)
}