package transport

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/JamesPrial/mcp-memory-core/pkg/config"
)

func TestHTTPTransport_HandleRPC(t *testing.T) {
	// Create test transport
	cfg := &config.TransportSettings{
		Host:             "localhost",
		Port:             8080,
		ReadTimeout:      30,
		WriteTimeout:     30,
		MaxConnections:   100,
		EnableCORS:       true,
		SSEHeartbeatSecs: 30,
	}
	
	transport := NewHTTPTransport(cfg)
	
	// Set up test handler
	testHandler := func(ctx context.Context, req *JSONRPCRequest) *JSONRPCResponse {
		if req.Method == "test/echo" {
			return &JSONRPCResponse{
				JSONRPC: "2.0",
				ID:      req.ID,
				Result:  map[string]interface{}{"echo": req.Params},
			}
		}
		return NewMethodNotFoundError(req.ID, req.Method)
	}
	
	transport.handler = testHandler
	
	// Test valid request
	t.Run("ValidRequest", func(t *testing.T) {
		reqBody := &JSONRPCRequest{
			JSONRPC: "2.0",
			ID:      1,
			Method:  "test/echo",
			Params:  map[string]interface{}{"message": "hello"},
		}
		
		body, _ := json.Marshal(reqBody)
		req := httptest.NewRequest(http.MethodPost, "/rpc", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		
		rr := httptest.NewRecorder()
		transport.handleRPC(rr, req)
		
		if rr.Code != http.StatusOK {
			t.Errorf("Expected status %d, got %d", http.StatusOK, rr.Code)
		}
		
		var resp JSONRPCResponse
		if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
			t.Fatalf("Failed to decode response: %v", err)
		}
		
		if resp.Error != nil {
			t.Errorf("Expected no error, got %v", resp.Error)
		}
	})
	
	// Test invalid JSON
	t.Run("InvalidJSON", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/rpc", bytes.NewReader([]byte("invalid json")))
		req.Header.Set("Content-Type", "application/json")
		
		rr := httptest.NewRecorder()
		transport.handleRPC(rr, req)
		
		if rr.Code != http.StatusOK {
			t.Errorf("Expected status %d, got %d", http.StatusOK, rr.Code)
		}
		
		var resp JSONRPCResponse
		if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
			t.Fatalf("Failed to decode response: %v", err)
		}
		
		if resp.Error == nil {
			t.Error("Expected error for invalid JSON")
		}
		if resp.Error.Code != ParseError {
			t.Errorf("Expected parse error code %d, got %d", ParseError, resp.Error.Code)
		}
	})
	
	// Test wrong method
	t.Run("WrongMethod", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/rpc", nil)
		
		rr := httptest.NewRecorder()
		transport.handleRPC(rr, req)
		
		if rr.Code != http.StatusMethodNotAllowed {
			t.Errorf("Expected status %d, got %d", http.StatusMethodNotAllowed, rr.Code)
		}
	})
}

func TestHTTPTransport_SessionManagement(t *testing.T) {
	cfg := &config.TransportSettings{
		Host:             "localhost",
		Port:             8080,
		ReadTimeout:      30,
		WriteTimeout:     30,
		MaxConnections:   100,
		EnableCORS:       false,
		SSEHeartbeatSecs: 30,
	}
	
	transport := NewHTTPTransport(cfg)
	transport.handler = func(ctx context.Context, req *JSONRPCRequest) *JSONRPCResponse {
		return &JSONRPCResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Result:  "ok",
		}
	}
	
	// Test session creation
	t.Run("SessionCreation", func(t *testing.T) {
		reqBody := &JSONRPCRequest{
			JSONRPC: "2.0",
			ID:      1,
			Method:  "test",
			Params:  map[string]interface{}{},
		}
		
		body, _ := json.Marshal(reqBody)
		req := httptest.NewRequest(http.MethodPost, "/rpc", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		
		rr := httptest.NewRecorder()
		transport.handleRPC(rr, req)
		
		sessionID := rr.Header().Get("X-Session-ID")
		if sessionID == "" {
			t.Error("Expected session ID to be created")
		}
	})
	
	// Test session reuse
	t.Run("SessionReuse", func(t *testing.T) {
		// Create initial session
		session, err := transport.sessionManager.CreateSession("http")
		if err != nil {
			t.Fatalf("Failed to create session: %v", err)
		}
		
		reqBody := &JSONRPCRequest{
			JSONRPC: "2.0",
			ID:      1,
			Method:  "test",
			Params:  map[string]interface{}{},
		}
		
		body, _ := json.Marshal(reqBody)
		req := httptest.NewRequest(http.MethodPost, "/rpc", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Session-ID", session.ID)
		
		rr := httptest.NewRecorder()
		transport.handleRPC(rr, req)
		
		newSessionID := rr.Header().Get("X-Session-ID")
		if newSessionID != "" {
			t.Error("Should not create new session when valid session provided")
		}
	})
}

func TestHTTPTransport_HealthCheck(t *testing.T) {
	cfg := &config.TransportSettings{
		Host:             "localhost",
		Port:             8080,
		ReadTimeout:      30,
		WriteTimeout:     30,
		MaxConnections:   100,
		EnableCORS:       false,
		SSEHeartbeatSecs: 30,
	}
	
	transport := NewHTTPTransport(cfg)
	
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rr := httptest.NewRecorder()
	
	transport.handleHealth(rr, req)
	
	if rr.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, rr.Code)
	}
	
	var health map[string]interface{}
	if err := json.NewDecoder(rr.Body).Decode(&health); err != nil {
		t.Fatalf("Failed to decode health response: %v", err)
	}
	
	if health["status"] != "healthy" {
		t.Errorf("Expected healthy status, got %v", health["status"])
	}
	
	if health["transport"] != "http" {
		t.Errorf("Expected http transport, got %v", health["transport"])
	}
}

func TestHTTPTransport_CORS(t *testing.T) {
	cfg := &config.TransportSettings{
		Host:             "localhost",
		Port:             8080,
		ReadTimeout:      30,
		WriteTimeout:     30,
		MaxConnections:   100,
		EnableCORS:       true,
		SSEHeartbeatSecs: 30,
	}
	
	transport := NewHTTPTransport(cfg)
	mux := http.NewServeMux()
	mux.HandleFunc("/test", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	
	handler := transport.corsMiddleware(mux)
	
	// Test preflight request
	t.Run("PreflightRequest", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodOptions, "/test", nil)
		rr := httptest.NewRecorder()
		
		handler.ServeHTTP(rr, req)
		
		if rr.Code != http.StatusNoContent {
			t.Errorf("Expected status %d, got %d", http.StatusNoContent, rr.Code)
		}
		
		expectedHeaders := map[string]string{
			"Access-Control-Allow-Origin":  "*",
			"Access-Control-Allow-Methods": "POST, GET, OPTIONS",
			"Access-Control-Allow-Headers": "Content-Type, X-Session-ID",
		}
		
		for header, expected := range expectedHeaders {
			if got := rr.Header().Get(header); got != expected {
				t.Errorf("Expected header %s: %s, got %s", header, expected, got)
			}
		}
	})
	
	// Test regular request with CORS
	t.Run("RegularRequestWithCORS", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		rr := httptest.NewRecorder()
		
		handler.ServeHTTP(rr, req)
		
		if rr.Header().Get("Access-Control-Allow-Origin") != "*" {
			t.Error("Expected CORS headers on regular request")
		}
	})
}