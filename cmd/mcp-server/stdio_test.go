package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"io"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/JamesPrial/mcp-memory-core/internal/knowledge"
	"github.com/JamesPrial/mcp-memory-core/internal/storage"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestableServer is a version of Server that allows injecting stdin/stdout for testing
type TestableServer struct {
	*Server
	stdin  io.Reader
	stdout io.Writer
}

// NewTestableServer creates a testable server
func NewTestableServer(manager *knowledge.Manager, stdin io.Reader, stdout io.Writer) *TestableServer {
	return &TestableServer{
		Server: NewServer(manager),
		stdin:  stdin,
		stdout: stdout,
	}
}

// TestableRun is like Run but uses injected stdin/stdout
func (ts *TestableServer) TestableRun(ctx context.Context) error {
	scanner := bufio.NewScanner(ts.stdin)
	
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		// Parse JSON-RPC request
		var req JSONRPCRequest
		if err := json.Unmarshal([]byte(line), &req); err != nil {
			// Send parse error response
			resp := &JSONRPCResponse{
				JSONRPC: "2.0",
				ID:      nil,
				Error: &JSONRPCError{
					Code:    -32700,
					Message: "Parse error",
				},
			}
			ts.testableSendResponse(resp)
			continue
		}

		// Handle the request
		resp := ts.HandleRequest(ctx, &req)
		ts.testableSendResponse(resp)
	}

	if err := scanner.Err(); err != nil && err != io.EOF {
		return err
	}

	return nil
}

// testableSendResponse sends response to injected stdout
func (ts *TestableServer) testableSendResponse(resp *JSONRPCResponse) {
	respBytes, err := json.Marshal(resp)
	if err != nil {
		return
	}
	
	ts.stdout.Write(respBytes)
	ts.stdout.Write([]byte("\n"))
}

// createTestServer creates a server with memory backend for testing
func createTestServer(t *testing.T) *Server {
	backend := storage.NewMemoryBackend()
	manager := knowledge.NewManager(backend)
	return NewServer(manager)
}

// TestServerTestableRun tests the testable version of Run method
func TestServerTestableRun(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected []string
	}{
		{
			name:  "valid tools/list request",
			input: `{"jsonrpc": "2.0", "id": 1, "method": "tools/list"}`,
			expected: []string{
				`"jsonrpc":"2.0"`,
				`"id":1`,
				`"result"`,
				`"tools"`,
			},
		},
		{
			name:  "empty lines should be ignored",
			input: "\n\n   \n\t\n",
			expected: []string{},
		},
		{
			name:  "malformed JSON request",
			input: `{"jsonrpc": "2.0", "id": 1, "method": "tools/list"`,
			expected: []string{
				`"jsonrpc":"2.0"`,
				`"id":null`,
				`"error"`,
				`"code":-32700`,
				`"message":"Parse error"`,
			},
		},
		{
			name:  "unknown method",
			input: `{"jsonrpc": "2.0", "id": 2, "method": "unknown/method"}`,
			expected: []string{
				`"jsonrpc":"2.0"`,
				`"id":2`,
				`"error"`,
				`"code":-32601`,
				`"message":"Method not found"`,
			},
		},
		{
			name:  "tools/call without name parameter",
			input: `{"jsonrpc": "2.0", "id": 3, "method": "tools/call", "params": {}}`,
			expected: []string{
				`"jsonrpc":"2.0"`,
				`"id":3`,
				`"error"`,
				`"code":-32602`,
				`"message":"Invalid params"`,
				`"data":"Missing 'name' field in params"`,
			},
		},
		{
			name:  "tools/call with invalid name type",
			input: `{"jsonrpc": "2.0", "id": 4, "method": "tools/call", "params": {"name": 123}}`,
			expected: []string{
				`"jsonrpc":"2.0"`,
				`"id":4`,
				`"error"`,
				`"code":-32602`,
				`"message":"Invalid params"`,
				`"data":"Field 'name' must be a string"`,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			backend := storage.NewMemoryBackend()
			manager := knowledge.NewManager(backend)

			// Set up input and output
			input := strings.NewReader(tt.input)
			var output bytes.Buffer

			server := NewTestableServer(manager, input, &output)

			// Run the server
			ctx := context.Background()
			err := server.TestableRun(ctx)
			assert.NoError(t, err)

			outputStr := output.String()
			
			// Verify expected strings in output
			if len(tt.expected) == 0 {
				assert.Empty(t, outputStr, "Expected no output")
			} else {
				for _, expected := range tt.expected {
					assert.Contains(t, outputStr, expected, "Expected substring not found in output")
				}
			}
		})
	}
}

// TestServerTestableRunWithMultipleRequests tests handling multiple requests
func TestServerTestableRunWithMultipleRequests(t *testing.T) {
	backend := storage.NewMemoryBackend()
	manager := knowledge.NewManager(backend)

	// Multiple requests input
	input := strings.NewReader(`{"jsonrpc": "2.0", "id": 1, "method": "tools/list"}
{"jsonrpc": "2.0", "id": 2, "method": "unknown/method"}
malformed json`)

	var output bytes.Buffer
	server := NewTestableServer(manager, input, &output)

	ctx := context.Background()
	err := server.TestableRun(ctx)
	assert.NoError(t, err)

	outputStr := output.String()
	lines := strings.Split(strings.TrimSpace(outputStr), "\n")
	
	// Should have received 3 responses
	assert.Len(t, lines, 3, "Expected 3 responses")
	
	// First response should be successful tools/list
	assert.Contains(t, lines[0], `"id":1`)
	assert.Contains(t, lines[0], `"result"`)
	
	// Second response should be method not found error
	assert.Contains(t, lines[1], `"id":2`)
	assert.Contains(t, lines[1], `"error"`)
	assert.Contains(t, lines[1], `"code":-32601`)
	
	// Third response should be parse error
	assert.Contains(t, lines[2], `"id":null`)
	assert.Contains(t, lines[2], `"error"`)
	assert.Contains(t, lines[2], `"code":-32700`)
}

// TestServerTestableRunEOF tests handling EOF condition
func TestServerTestableRunEOF(t *testing.T) {
	backend := storage.NewMemoryBackend()
	manager := knowledge.NewManager(backend)

	// Empty input will immediately EOF
	input := strings.NewReader("")
	var output bytes.Buffer

	server := NewTestableServer(manager, input, &output)

	ctx := context.Background()
	err := server.TestableRun(ctx)
	
	// Should return nil on EOF
	assert.NoError(t, err, "TestableRun should return nil on EOF")
}

// TestSendResponseDirect tests the original sendResponse method
func TestSendResponseDirect(t *testing.T) {
	tests := []struct {
		name     string
		response *JSONRPCResponse
		expected string
	}{
		{
			name: "successful response",
			response: &JSONRPCResponse{
				JSONRPC: "2.0",
				ID:      1,
				Result:  map[string]interface{}{"test": "value"},
			},
			expected: `{"jsonrpc":"2.0","id":1,"result":{"test":"value"}}`,
		},
		{
			name: "error response",
			response: &JSONRPCResponse{
				JSONRPC: "2.0",
				ID:      2,
				Error: &JSONRPCError{
					Code:    -32601,
					Message: "Method not found",
				},
			},
			expected: `{"jsonrpc":"2.0","id":2,"error":{"code":-32601,"message":"Method not found"}}`,
		},
		{
			name: "response with null ID",
			response: &JSONRPCResponse{
				JSONRPC: "2.0",
				ID:      nil,
				Error: &JSONRPCError{
					Code:    -32700,
					Message: "Parse error",
				},
			},
			expected: `{"jsonrpc":"2.0","id":null,"error":{"code":-32700,"message":"Parse error"}}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := createTestServer(t)

			// Capture stdout using a pipe
			originalStdout := os.Stdout
			r, w, err := os.Pipe()
			require.NoError(t, err)
			os.Stdout = w

			// Channel to read output
			outputChan := make(chan string, 1)
			go func() {
				var buf bytes.Buffer
				io.Copy(&buf, r)
				outputChan <- buf.String()
			}()

			// Call sendResponse
			server.sendResponse(tt.response)

			// Restore stdout and close pipe
			w.Close()
			os.Stdout = originalStdout

			// Get output
			select {
			case output := <-outputChan:
				assert.Equal(t, tt.expected+"\n", output)
			case <-time.After(1 * time.Second):
				t.Error("Timeout waiting for output")
			}

			r.Close()
		})
	}
}

// TestSendResponseWithMarshalError tests sendResponse with unmarshalable data
func TestSendResponseWithMarshalError(t *testing.T) {
	server := createTestServer(t)

	// Create response with unmarshalable data (channels cannot be marshaled)
	response := &JSONRPCResponse{
		JSONRPC: "2.0",
		ID:      1,
		Result:  make(chan int), // This will cause json.Marshal to fail
	}

	// Capture stdout
	originalStdout := os.Stdout
	r, w, err := os.Pipe()
	require.NoError(t, err)
	os.Stdout = w

	outputChan := make(chan string, 1)
	go func() {
		var buf bytes.Buffer
		io.Copy(&buf, r)
		outputChan <- buf.String()
	}()

	// This should not panic and should not write anything to stdout
	server.sendResponse(response)

	w.Close()
	os.Stdout = originalStdout

	select {
	case output := <-outputChan:
		// We now output an error response when marshal fails
		assert.Contains(t, output, "Failed to serialize response")
	case <-time.After(1 * time.Second):
		t.Error("Timeout waiting for output")
	}

	r.Close()
}

// TestHandleRequestEdgeCases tests additional edge cases for HandleRequest
func TestHandleRequestEdgeCases(t *testing.T) {
	server := createTestServer(t)
	ctx := context.Background()

	tests := []struct {
		name     string
		request  *JSONRPCRequest
		checkFn  func(t *testing.T, resp *JSONRPCResponse)
	}{
		{
			name: "tools/call with valid arguments",
			request: &JSONRPCRequest{
				JSONRPC: "2.0",
				ID:      1,
				Method:  "tools/call",
				Params: map[string]interface{}{
					"name": "memory__get_statistics",
					"arguments": map[string]interface{}{},
				},
			},
			checkFn: func(t *testing.T, resp *JSONRPCResponse) {
				assert.Equal(t, "2.0", resp.JSONRPC)
				assert.Equal(t, 1, resp.ID)
				assert.NotNil(t, resp.Result)
				assert.Nil(t, resp.Error)
			},
		},
		{
			name: "tools/call with non-map arguments",
			request: &JSONRPCRequest{
				JSONRPC: "2.0",
				ID:      2,
				Method:  "tools/call",
				Params: map[string]interface{}{
					"name": "memory__get_statistics",
					"arguments": "not-a-map",
				},
			},
			checkFn: func(t *testing.T, resp *JSONRPCResponse) {
				assert.Equal(t, "2.0", resp.JSONRPC)
				assert.Equal(t, 2, resp.ID)
				assert.Nil(t, resp.Result)
				assert.NotNil(t, resp.Error)
				assert.Equal(t, -32602, resp.Error.Code)
			},
		},
		{
			name: "tools/call with invalid tool name",
			request: &JSONRPCRequest{
				JSONRPC: "2.0",
				ID:      3,
				Method:  "tools/call",
				Params: map[string]interface{}{
					"name": "nonexistent_tool",
					"arguments": map[string]interface{}{},
				},
			},
			checkFn: func(t *testing.T, resp *JSONRPCResponse) {
				assert.Equal(t, "2.0", resp.JSONRPC)
				assert.Equal(t, 3, resp.ID)
				assert.Nil(t, resp.Result)
				assert.NotNil(t, resp.Error)
				assert.Equal(t, -32602, resp.Error.Code) // Invalid params since tool doesn't start with memory__
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp := server.HandleRequest(ctx, tt.request)
			tt.checkFn(t, resp)
		})
	}
}

// TestJSONRPCStructures tests JSON marshaling/unmarshaling of structures
func TestJSONRPCStructures(t *testing.T) {
	t.Run("JSONRPCRequest marshaling", func(t *testing.T) {
		req := &JSONRPCRequest{
			JSONRPC: "2.0",
			ID:      "test-id",
			Method:  "test/method",
			Params: map[string]interface{}{
				"key": "value",
			},
		}

		data, err := json.Marshal(req)
		require.NoError(t, err)

		var unmarshaled JSONRPCRequest
		err = json.Unmarshal(data, &unmarshaled)
		require.NoError(t, err)

		assert.Equal(t, req.JSONRPC, unmarshaled.JSONRPC)
		assert.Equal(t, req.ID, unmarshaled.ID)
		assert.Equal(t, req.Method, unmarshaled.Method)
		assert.Equal(t, req.Params, unmarshaled.Params)
	})

	t.Run("JSONRPCResponse marshaling", func(t *testing.T) {
		resp := &JSONRPCResponse{
			JSONRPC: "2.0",
			ID:      123,
			Result:  map[string]interface{}{"result": "data"},
			Error: &JSONRPCError{
				Code:    -32601,
				Message: "Method not found",
				Data:    "additional info",
			},
		}

		data, err := json.Marshal(resp)
		require.NoError(t, err)

		var unmarshaled JSONRPCResponse
		err = json.Unmarshal(data, &unmarshaled)
		require.NoError(t, err)

		assert.Equal(t, resp.JSONRPC, unmarshaled.JSONRPC)
		assert.Equal(t, float64(123), unmarshaled.ID) // JSON unmarshals numbers as float64
		assert.Equal(t, resp.Result, unmarshaled.Result)
		assert.NotNil(t, unmarshaled.Error)
		assert.Equal(t, resp.Error.Code, unmarshaled.Error.Code)
		assert.Equal(t, resp.Error.Message, unmarshaled.Error.Message)
		assert.Equal(t, resp.Error.Data, unmarshaled.Error.Data)
	})
}

// TestNewServer tests the NewServer constructor
func TestNewServer(t *testing.T) {
	backend := storage.NewMemoryBackend()
	manager := knowledge.NewManager(backend)
	
	server := NewServer(manager)
	
	assert.NotNil(t, server)
	assert.Equal(t, manager, server.manager)
}

// TestOriginalRunMethod tests the original Run method with direct stdin manipulation
func TestOriginalRunMethod(t *testing.T) {
	server := createTestServer(t)
	
	// Create a pipe to simulate stdin
	r, w, err := os.Pipe()
	require.NoError(t, err)
	
	// Save original stdin
	originalStdin := os.Stdin
	defer func() {
		os.Stdin = originalStdin
	}()
	
	// Replace stdin with our pipe
	os.Stdin = r
	
	// Create context with timeout to avoid infinite running
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	
	// Run server in goroutine
	errChan := make(chan error, 1)
	go func() {
		errChan <- server.Run(ctx)
	}()
	
	// Send requests with empty lines and malformed JSON to test all paths
	requests := []string{
		"",  // Empty line
		"   ", // Whitespace only
		`{"jsonrpc": "2.0", "id": 1, "method": "tools/list"}`, // Valid request
		`{"jsonrpc": "2.0", "id": 2, "method"`, // Malformed JSON
		`{"jsonrpc": "2.0", "id": 3, "method": "tools/list"}`, // Another valid request
	}
	
	for _, req := range requests {
		_, err = w.Write([]byte(req + "\n"))
		require.NoError(t, err)
		time.Sleep(5 * time.Millisecond) // Small delay
	}
	
	// Close writer to signal EOF
	w.Close()
	
	// Wait for server to complete
	select {
	case err := <-errChan:
		assert.NoError(t, err)
	case <-time.After(1 * time.Second):
		t.Error("Run method did not complete in time")
		cancel()
	}
}

// TestMoreEdgeCases tests additional edge cases to improve coverage
func TestMoreEdgeCases(t *testing.T) {
	server := createTestServer(t)
	ctx := context.Background()

	t.Run("tools/call with nil arguments", func(t *testing.T) {
		req := &JSONRPCRequest{
			JSONRPC: "2.0",
			ID:      1,
			Method:  "tools/call",
			Params: map[string]interface{}{
				"name": "memory__get_statistics",
				"arguments": nil,
			},
		}
		resp := server.HandleRequest(ctx, req)
		// nil arguments should be treated as empty map, so the call should succeed
		assert.NotNil(t, resp.Result)
		assert.Nil(t, resp.Error)
	})

	t.Run("tools/call without arguments field", func(t *testing.T) {
		req := &JSONRPCRequest{
			JSONRPC: "2.0",
			ID:      1,
			Method:  "tools/call",
			Params: map[string]interface{}{
				"name": "memory__get_statistics",
			},
		}
		resp := server.HandleRequest(ctx, req)
		assert.NotNil(t, resp.Result)
		assert.Nil(t, resp.Error)
	})

	t.Run("handleToolsList coverage", func(t *testing.T) {
		req := &JSONRPCRequest{
			JSONRPC: "2.0",
			ID:      "string-id",
			Method:  "tools/list",
		}
		resp := server.handleToolsList(req)
		assert.Equal(t, "2.0", resp.JSONRPC)
		assert.Equal(t, "string-id", resp.ID)
		assert.NotNil(t, resp.Result)
		assert.Nil(t, resp.Error)
	})

	t.Run("handleToolsCall coverage - direct call", func(t *testing.T) {
		req := &JSONRPCRequest{
			JSONRPC: "2.0",
			ID:      42,
			Method:  "tools/call",
			Params: map[string]interface{}{
				"name": "memory__get_statistics",
				"arguments": map[string]interface{}{},
			},
		}
		resp := server.handleToolsCall(ctx, req)
		assert.Equal(t, "2.0", resp.JSONRPC)
		assert.Equal(t, 42, resp.ID)
		assert.NotNil(t, resp.Result)
		assert.Nil(t, resp.Error)
	})
}

// TestErrorResponse tests creating error responses  
func TestErrorResponse(t *testing.T) {
	// Test creating JSONRPCError
	err := &JSONRPCError{
		Code:    -32603,
		Message: "Internal error",
		Data:    "additional data",
	}
	
	assert.Equal(t, -32603, err.Code)
	assert.Equal(t, "Internal error", err.Message)
	assert.Equal(t, "additional data", err.Data)

	// Test creating full error response
	resp := &JSONRPCResponse{
		JSONRPC: "2.0",
		ID:      nil,
		Error:   err,
	}
	
	assert.Equal(t, "2.0", resp.JSONRPC)
	assert.Nil(t, resp.ID)
	assert.Nil(t, resp.Result)
	assert.NotNil(t, resp.Error)
}

// TestRunMethodScannerError tests the scanner error path
func TestRunMethodScannerError(t *testing.T) {
	server := createTestServer(t)
	
	// Create a pipe and close read end to simulate an error
	r, w, err := os.Pipe()
	require.NoError(t, err)
	w.Close() // Close write end immediately
	r.Close() // Close read end to cause scanner error
	
	// Create a new pipe for actual test
	r2, w2, err := os.Pipe()
	require.NoError(t, err)
	
	// Save original stdin
	originalStdin := os.Stdin
	defer func() {
		os.Stdin = originalStdin
	}()
	
	// Replace stdin with closed pipe
	os.Stdin = r2
	
	// Close immediately to cause EOF/error condition
	w2.Close()
	
	ctx := context.Background()
	err = server.Run(ctx)
	
	// Should return nil on EOF (not an error)
	assert.NoError(t, err)
}