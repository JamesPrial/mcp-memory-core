package transport

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/JamesPrial/mcp-memory-core/pkg/config"
)

func TestSSETransport_HandleSSE(t *testing.T) {
	cfg := &config.TransportSettings{
		Host:             "localhost",
		Port:             8080,
		ReadTimeout:      30,
		WriteTimeout:     30,
		MaxConnections:   100,
		EnableCORS:       false,
		SSEHeartbeatSecs: 1, // Short heartbeat for testing
	}
	
	transport := NewSSETransport(cfg)
	transport.handler = func(ctx context.Context, req *JSONRPCRequest) *JSONRPCResponse {
		return &JSONRPCResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Result:  "ok",
		}
	}
	
	// Test SSE connection establishment
	t.Run("SSEConnection", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/events", nil)
		rr := httptest.NewRecorder()
		
		// Use a context with timeout to prevent hanging
		ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
		defer cancel()
		req = req.WithContext(ctx)
		
		go transport.handleSSE(rr, req)
		
		// Give it time to write initial events
		time.Sleep(50 * time.Millisecond)
		
		body := rr.Body.String()
		
		// Check for SSE headers
		if ct := rr.Header().Get("Content-Type"); ct != "text/event-stream" {
			t.Errorf("Expected Content-Type text/event-stream, got %s", ct)
		}
		
		// Check for session event
		if !strings.Contains(body, "event: session") {
			t.Error("Expected session event in SSE stream")
		}
		
		// Check for connected event
		if !strings.Contains(body, "event: connected") {
			t.Error("Expected connected event in SSE stream")
		}
	})
	
	// Test SSE with existing session
	t.Run("SSEWithExistingSession", func(t *testing.T) {
		// Create a session first
		session, err := transport.sessionManager.CreateSession("sse")
		if err != nil {
			t.Fatalf("Failed to create session: %v", err)
		}
		
		req := httptest.NewRequest(http.MethodGet, "/events", nil)
		req.Header.Set("X-Session-ID", session.ID)
		rr := httptest.NewRecorder()
		
		ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
		defer cancel()
		req = req.WithContext(ctx)
		
		go transport.handleSSE(rr, req)
		
		time.Sleep(50 * time.Millisecond)
		
		body := rr.Body.String()
		
		// Should not create new session
		if strings.Contains(body, "event: session") {
			t.Error("Should not send session event for existing session")
		}
		
		// Should still send connected event
		if !strings.Contains(body, "event: connected") {
			t.Error("Expected connected event in SSE stream")
		}
	})
}

func TestSSETransport_HandleRPC(t *testing.T) {
	cfg := &config.TransportSettings{
		Host:             "localhost",
		Port:             8080,
		ReadTimeout:      30,
		WriteTimeout:     30,
		MaxConnections:   100,
		EnableCORS:       false,
		SSEHeartbeatSecs: 30,
	}
	
	transport := NewSSETransport(cfg)
	transport.handler = func(ctx context.Context, req *JSONRPCRequest) *JSONRPCResponse {
		return &JSONRPCResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Result:  map[string]interface{}{"echo": req.Params},
		}
	}
	
	// Test RPC without session
	t.Run("RPCWithoutSession", func(t *testing.T) {
		reqBody := &JSONRPCRequest{
			JSONRPC: "2.0",
			ID:      1,
			Method:  "test",
			Params:  map[string]interface{}{"message": "hello"},
		}
		
		body, _ := json.Marshal(reqBody)
		req := httptest.NewRequest(http.MethodPost, "/rpc", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		
		rr := httptest.NewRecorder()
		transport.handleRPC(rr, req)
		
		if rr.Code != http.StatusBadRequest {
			t.Errorf("Expected status %d, got %d", http.StatusBadRequest, rr.Code)
		}
	})
	
	// Test RPC with valid session
	t.Run("RPCWithValidSession", func(t *testing.T) {
		// Create session and queue
		session, err := transport.sessionManager.CreateSession("sse")
		if err != nil {
			t.Fatalf("Failed to create session: %v", err)
		}
		
		// Create message queue for session
		transport.queuesMu.Lock()
		transport.messageQueues[session.ID] = make(chan *JSONRPCResponse, 1)
		transport.queuesMu.Unlock()
		
		reqBody := &JSONRPCRequest{
			JSONRPC: "2.0",
			ID:      1,
			Method:  "test",
			Params:  map[string]interface{}{"message": "hello"},
		}
		
		body, _ := json.Marshal(reqBody)
		req := httptest.NewRequest(http.MethodPost, "/rpc", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Session-ID", session.ID)
		
		rr := httptest.NewRecorder()
		transport.handleRPC(rr, req)
		
		if rr.Code != http.StatusAccepted {
			t.Errorf("Expected status %d, got %d", http.StatusAccepted, rr.Code)
		}
		
		// Check if response was queued
		select {
		case msg := <-transport.messageQueues[session.ID]:
			if msg == nil {
				t.Error("Expected message in queue")
			}
			if msg.Error != nil {
				t.Errorf("Expected no error, got %v", msg.Error)
			}
		case <-time.After(100 * time.Millisecond):
			t.Error("No message queued")
		}
	})
	
	// Test RPC with invalid session
	t.Run("RPCWithInvalidSession", func(t *testing.T) {
		reqBody := &JSONRPCRequest{
			JSONRPC: "2.0",
			ID:      1,
			Method:  "test",
			Params:  map[string]interface{}{"message": "hello"},
		}
		
		body, _ := json.Marshal(reqBody)
		req := httptest.NewRequest(http.MethodPost, "/rpc", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Session-ID", "invalid-session-id")
		
		rr := httptest.NewRecorder()
		transport.handleRPC(rr, req)
		
		if rr.Code != http.StatusUnauthorized {
			t.Errorf("Expected status %d, got %d", http.StatusUnauthorized, rr.Code)
		}
	})
}

func TestSSETransport_BroadcastEvent(t *testing.T) {
	cfg := &config.TransportSettings{
		Host:             "localhost",
		Port:             8080,
		ReadTimeout:      30,
		WriteTimeout:     30,
		MaxConnections:   100,
		EnableCORS:       false,
		SSEHeartbeatSecs: 30,
	}
	
	transport := NewSSETransport(cfg)
	
	// Create mock clients
	client1 := &SSEClient{
		SessionID:    "session1",
		EventChannel: make(chan *SSEEvent, 1),
		Done:         make(chan bool),
	}
	
	client2 := &SSEClient{
		SessionID:    "session2",
		EventChannel: make(chan *SSEEvent, 1),
		Done:         make(chan bool),
	}
	
	transport.clientsMu.Lock()
	transport.clients["session1"] = client1
	transport.clients["session2"] = client2
	transport.clientsMu.Unlock()
	
	// Broadcast event
	event := &SSEEvent{
		ID:    "1",
		Event: "broadcast",
		Data:  "test message",
	}
	
	transport.BroadcastEvent(event)
	
	// Check both clients received the event
	for _, client := range []*SSEClient{client1, client2} {
		select {
		case received := <-client.EventChannel:
			if received.ID != event.ID {
				t.Errorf("Expected event ID %s, got %s", event.ID, received.ID)
			}
			if received.Data != event.Data {
				t.Errorf("Expected event data %s, got %s", event.Data, received.Data)
			}
		case <-time.After(100 * time.Millisecond):
			t.Errorf("Client %s did not receive broadcast event", client.SessionID)
		}
	}
}

func TestSSETransport_ParseSSEStream(t *testing.T) {
	// Helper function to parse SSE stream
	parseSSE := func(data string) []map[string]string {
		var events []map[string]string
		scanner := bufio.NewScanner(strings.NewReader(data))
		
		currentEvent := make(map[string]string)
		for scanner.Scan() {
			line := scanner.Text()
			if line == "" {
				if len(currentEvent) > 0 {
					events = append(events, currentEvent)
					currentEvent = make(map[string]string)
				}
				continue
			}
			
			if strings.HasPrefix(line, ":") {
				// Comment line
				continue
			}
			
			parts := strings.SplitN(line, ":", 2)
			if len(parts) == 2 {
				field := strings.TrimSpace(parts[0])
				value := strings.TrimSpace(parts[1])
				currentEvent[field] = value
			}
		}
		
		if len(currentEvent) > 0 {
			events = append(events, currentEvent)
		}
		
		return events
	}
	
	// Test SSE stream format
	testData := `event: message
data: {"test": "data"}

event: ping
data: pong

:heartbeat

event: close
data: goodbye
`
	
	events := parseSSE(testData)
	
	if len(events) != 3 {
		t.Errorf("Expected 3 events, got %d", len(events))
	}
	
	// Check first event
	if events[0]["event"] != "message" {
		t.Errorf("Expected event type 'message', got %s", events[0]["event"])
	}
	
	// Check second event
	if events[1]["data"] != "pong" {
		t.Errorf("Expected data 'pong', got %s", events[1]["data"])
	}
	
	// Check third event
	if events[2]["event"] != "close" {
		t.Errorf("Expected event type 'close', got %s", events[2]["event"])
	}
}