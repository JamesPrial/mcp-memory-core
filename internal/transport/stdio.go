package transport

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
)

// StdioTransport implements the Transport interface for stdio communication
type StdioTransport struct {
	scanner *bufio.Scanner
	running bool
}

// NewStdioTransport creates a new stdio transport
func NewStdioTransport() *StdioTransport {
	return &StdioTransport{
		scanner: bufio.NewScanner(os.Stdin),
	}
}

// Start begins listening for requests on stdin
func (t *StdioTransport) Start(ctx context.Context, handler RequestHandler) error {
	t.running = true
	
	for t.running && t.scanner.Scan() {
		// Check context cancellation
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		
		line := strings.TrimSpace(t.scanner.Text())
		if line == "" {
			continue
		}
		
		// Parse JSON-RPC request
		var req JSONRPCRequest
		if err := json.Unmarshal([]byte(line), &req); err != nil {
			// Send parse error response
			resp := NewParseError()
			t.sendResponse(resp)
			continue
		}
		
		// Handle the request
		resp := handler(ctx, &req)
		t.sendResponse(resp)
	}
	
	if err := t.scanner.Err(); err != nil && err != io.EOF {
		return fmt.Errorf("error reading from stdin: %w", err)
	}
	
	return nil
}

// Stop gracefully shuts down the transport
func (t *StdioTransport) Stop(ctx context.Context) error {
	t.running = false
	return nil
}

// Name returns the name of the transport
func (t *StdioTransport) Name() string {
	return "stdio"
}

// sendResponse sends a JSON-RPC response to stdout
func (t *StdioTransport) sendResponse(resp *JSONRPCResponse) {
	respBytes, err := json.Marshal(resp)
	if err != nil {
		// Send a generic error response if marshaling fails
		fallbackResp := NewInternalError(nil, "Failed to serialize response")
		if fallbackBytes, fallbackErr := json.Marshal(fallbackResp); fallbackErr == nil {
			fmt.Println(string(fallbackBytes))
		}
		return
	}
	
	fmt.Println(string(respBytes))
}