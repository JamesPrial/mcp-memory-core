package transport

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/JamesPrial/mcp-memory-core/pkg/config"
)

// TestSessionManagerRace tests concurrent access to SessionManager
func TestSessionManagerRace(t *testing.T) {
	sm := NewSessionManager(30 * time.Minute)
	defer sm.Stop()

	// Create initial session
	session, err := sm.CreateSession("test")
	if err != nil {
		t.Fatalf("Failed to create session: %v", err)
	}
	sessionID := session.ID

	// Start multiple goroutines accessing the session concurrently
	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 100; j++ {
				// Try to get session - this updates LastActivity
				sm.GetSession(sessionID)
				
				// Sometimes create new sessions
				if j%10 == 0 {
					session, _ := sm.CreateSession("test")
					sm.RemoveSession(session.ID)
				}
			}
		}()
	}

	// Goroutine that tries to remove sessions
	wg.Add(1)
	go func() {
		defer wg.Done()
		time.Sleep(10 * time.Millisecond)
		for i := 0; i < 50; i++ {
			session, _ := sm.CreateSession("test")
			time.Sleep(time.Millisecond)
			sm.RemoveSession(session.ID)
		}
	}()

	wg.Wait()
}

// TestHTTPTransportHandlerRace tests concurrent access to HTTPTransport handler
func TestHTTPTransportHandlerRace(t *testing.T) {
	cfg := &config.TransportSettings{
		Host:         "localhost",
		Port:         0, // Use any available port
		ReadTimeout:  10,
		WriteTimeout: 10,
	}
	
	transport := NewHTTPTransport(cfg)
	
	// Mock handler
	handler := func(ctx context.Context, req *JSONRPCRequest) *JSONRPCResponse {
		return &JSONRPCResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Result:  "test",
		}
	}

	var wg sync.WaitGroup
	
	// Start transport in multiple goroutines (simulating restarts)
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
			defer cancel()
			transport.Start(ctx, handler)
		}()
	}
	
	// Simulate concurrent requests
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 50; j++ {
				// Try to use handler through handleRPC
				// This would normally be called through HTTP requests
				time.Sleep(time.Microsecond)
			}
		}()
	}
	
	wg.Wait()
}

// TestSSETransportHandlerRace tests concurrent access to SSETransport handler
func TestSSETransportHandlerRace(t *testing.T) {
	cfg := &config.TransportSettings{
		Host:         "localhost",
		Port:         0, // Use any available port
		ReadTimeout:  10,
		WriteTimeout: 10,
	}
	
	transport := NewSSETransport(cfg)
	
	// Mock handler
	handler := func(ctx context.Context, req *JSONRPCRequest) *JSONRPCResponse {
		return &JSONRPCResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Result:  "test",
		}
	}

	var wg sync.WaitGroup
	
	// Start transport in multiple goroutines (simulating restarts)
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
			defer cancel()
			transport.Start(ctx, handler)
		}()
	}
	
	// Simulate concurrent SSE operations
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			sessionID := fmt.Sprintf("session-%d", id)
			
			// Simulate message sending
			for j := 0; j < 20; j++ {
				resp := &JSONRPCResponse{
					JSONRPC: "2.0",
					ID:      j,
					Result:  "test",
				}
				transport.sendSSEResponse(sessionID, resp)
				time.Sleep(time.Microsecond)
			}
		}(i)
	}
	
	// Simulate broadcast events
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 30; i++ {
			event := &SSEEvent{
				ID:    fmt.Sprintf("event-%d", i),
				Event: "test",
				Data:  "test data",
			}
			transport.BroadcastEvent(event)
			time.Sleep(time.Microsecond)
		}
	}()
	
	wg.Wait()
}

// TestSessionStoreRace tests concurrent access to SessionStore
func TestSessionStoreRace(t *testing.T) {
	store := NewSessionStore()
	
	var wg sync.WaitGroup
	
	// Multiple writers
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < 100; j++ {
				key := fmt.Sprintf("key-%d", j%5)
				store.Set(key, fmt.Sprintf("value-%d-%d", id, j))
			}
		}(i)
	}
	
	// Multiple readers
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 100; j++ {
				key := fmt.Sprintf("key-%d", j%5)
				store.Get(key)
			}
		}()
	}
	
	wg.Wait()
}