package transport

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"

	"github.com/JamesPrial/mcp-memory-core/pkg/config"
)

// SSEClient represents a connected SSE client
type SSEClient struct {
	SessionID    string
	EventChannel chan *SSEEvent
	Done         chan bool
}

// SSEEvent represents a Server-Sent Event
type SSEEvent struct {
	ID    string
	Event string
	Data  string
}

// SSETransport implements the Transport interface for SSE communication
type SSETransport struct {
	config         *config.TransportSettings
	server         *http.Server
	handler        RequestHandler
	sessionManager *SessionManager
	clients        map[string]*SSEClient
	clientsMu      sync.RWMutex
	messageQueues  map[string]chan *JSONRPCResponse
	queuesMu       sync.RWMutex
}

// NewSSETransport creates a new SSE transport
func NewSSETransport(cfg *config.TransportSettings) *SSETransport {
	return &SSETransport{
		config:         cfg,
		sessionManager: NewSessionManager(30 * time.Minute),
		clients:        make(map[string]*SSEClient),
		messageQueues:  make(map[string]chan *JSONRPCResponse),
	}
}

// Start begins listening for SSE connections
func (t *SSETransport) Start(ctx context.Context, handler RequestHandler) error {
	t.handler = handler
	
	mux := http.NewServeMux()
	
	// Main RPC endpoint (accepts POST requests)
	mux.HandleFunc("/rpc", t.handleRPC)
	
	// SSE events endpoint
	mux.HandleFunc("/events", t.handleSSE)
	
	// Health check endpoint
	mux.HandleFunc("/health", t.handleHealth)
	
	// CORS middleware
	var httpHandler http.Handler = mux
	if t.config.EnableCORS {
		httpHandler = t.corsMiddleware(mux)
	}
	
	// Create HTTP server
	addr := fmt.Sprintf("%s:%d", t.config.Host, t.config.Port)
	t.server = &http.Server{
		Addr:         addr,
		Handler:      httpHandler,
		ReadTimeout:  time.Duration(t.config.ReadTimeout) * time.Second,
		WriteTimeout: time.Duration(t.config.WriteTimeout) * time.Second,
	}
	
	// Start server in goroutine
	errChan := make(chan error, 1)
	go func() {
		if err := t.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			errChan <- err
		}
	}()
	
	// Wait for context cancellation or server error
	select {
	case <-ctx.Done():
		return t.Stop(context.Background())
	case err := <-errChan:
		return err
	}
}

// Stop gracefully shuts down the SSE server
func (t *SSETransport) Stop(ctx context.Context) error {
	// Close all client connections
	t.clientsMu.Lock()
	for _, client := range t.clients {
		close(client.Done)
	}
	t.clientsMu.Unlock()
	
	if t.server != nil {
		t.sessionManager.Stop()
		return t.server.Shutdown(ctx)
	}
	return nil
}

// Name returns the name of the transport
func (t *SSETransport) Name() string {
	return "sse"
}

// handleRPC handles JSON-RPC requests via HTTP POST
func (t *SSETransport) handleRPC(w http.ResponseWriter, r *http.Request) {
	// Only accept POST requests
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	
	// Check content type
	contentType := r.Header.Get("Content-Type")
	if contentType != "application/json" && contentType != "application/json-rpc" {
		http.Error(w, "Content-Type must be application/json or application/json-rpc", http.StatusBadRequest)
		return
	}
	
	// Get session ID
	sessionID := r.Header.Get("X-Session-ID")
	if sessionID == "" {
		http.Error(w, "X-Session-ID header required", http.StatusBadRequest)
		return
	}
	
	// Check if session exists
	if _, exists := t.sessionManager.GetSession(sessionID); !exists {
		http.Error(w, "Invalid or expired session", http.StatusUnauthorized)
		return
	}
	
	// Read request body
	body, err := io.ReadAll(io.LimitReader(r.Body, 10*1024*1024)) // 10MB limit
	if err != nil {
		http.Error(w, "Failed to read request body", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()
	
	// Parse JSON-RPC request
	var req JSONRPCRequest
	if err := json.Unmarshal(body, &req); err != nil {
		t.sendSSEResponse(sessionID, NewParseError())
		w.WriteHeader(http.StatusAccepted)
		return
	}
	
	// Handle the request
	ctx := r.Context()
	resp := t.handler(ctx, &req)
	
	// Send response via SSE
	t.sendSSEResponse(sessionID, resp)
	
	// Return 202 Accepted to indicate the request was received
	w.WriteHeader(http.StatusAccepted)
	w.Write([]byte(`{"status":"accepted"}`))
}

// handleSSE handles SSE connections
func (t *SSETransport) handleSSE(w http.ResponseWriter, r *http.Request) {
	// Check if SSE is supported
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "SSE not supported", http.StatusInternalServerError)
		return
	}
	
	// Set SSE headers
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no") // Disable Nginx buffering
	
	// Get or create session
	sessionID := r.Header.Get("X-Session-ID")
	lastEventID := r.Header.Get("Last-Event-ID")
	
	if sessionID == "" {
		session, err := t.sessionManager.CreateSession("sse")
		if err != nil {
			http.Error(w, "Failed to create session", http.StatusInternalServerError)
			return
		}
		sessionID = session.ID
		
		// Send session ID as first event
		fmt.Fprintf(w, "event: session\ndata: {\"sessionId\":\"%s\"}\n\n", sessionID)
		flusher.Flush()
	} else {
		if _, exists := t.sessionManager.GetSession(sessionID); !exists {
			// Session expired, create new one
			session, err := t.sessionManager.CreateSession("sse")
			if err != nil {
				http.Error(w, "Failed to create session", http.StatusInternalServerError)
				return
			}
			sessionID = session.ID
			fmt.Fprintf(w, "event: session\ndata: {\"sessionId\":\"%s\"}\n\n", sessionID)
			flusher.Flush()
		}
	}
	
	// Create client
	client := &SSEClient{
		SessionID:    sessionID,
		EventChannel: make(chan *SSEEvent, 10),
		Done:         make(chan bool),
	}
	
	// Register client
	t.clientsMu.Lock()
	t.clients[sessionID] = client
	t.clientsMu.Unlock()
	
	// Create message queue for this session
	t.queuesMu.Lock()
	if _, exists := t.messageQueues[sessionID]; !exists {
		t.messageQueues[sessionID] = make(chan *JSONRPCResponse, 100)
	}
	msgQueue := t.messageQueues[sessionID]
	t.queuesMu.Unlock()
	
	// Clean up on disconnect
	defer func() {
		t.clientsMu.Lock()
		delete(t.clients, sessionID)
		t.clientsMu.Unlock()
		close(client.EventChannel)
	}()
	
	// Send initial connection event
	fmt.Fprintf(w, "event: connected\ndata: {\"status\":\"connected\"}\n\n")
	flusher.Flush()
	
	// Handle reconnection with Last-Event-ID
	if lastEventID != "" {
		fmt.Fprintf(w, "event: reconnected\ndata: {\"lastEventId\":\"%s\"}\n\n", lastEventID)
		flusher.Flush()
	}
	
	// Start heartbeat
	heartbeat := time.NewTicker(time.Duration(t.config.SSEHeartbeatSecs) * time.Second)
	defer heartbeat.Stop()
	
	// Event loop
	for {
		select {
		case <-r.Context().Done():
			return
			
		case <-client.Done:
			return
			
		case <-heartbeat.C:
			// Send heartbeat
			fmt.Fprintf(w, ":heartbeat\n\n")
			flusher.Flush()
			
		case msg := <-msgQueue:
			// Send JSON-RPC response as SSE event
			if msg != nil {
				data, err := json.Marshal(msg)
				if err != nil {
					continue
				}
				fmt.Fprintf(w, "event: message\ndata: %s\n\n", string(data))
				flusher.Flush()
			}
			
		case event := <-client.EventChannel:
			// Send custom event
			if event != nil {
				if event.ID != "" {
					fmt.Fprintf(w, "id: %s\n", event.ID)
				}
				if event.Event != "" {
					fmt.Fprintf(w, "event: %s\n", event.Event)
				}
				fmt.Fprintf(w, "data: %s\n\n", event.Data)
				flusher.Flush()
			}
		}
	}
}

// handleHealth handles health check requests
func (t *SSETransport) handleHealth(w http.ResponseWriter, r *http.Request) {
	t.clientsMu.RLock()
	clientCount := len(t.clients)
	t.clientsMu.RUnlock()
	
	health := map[string]interface{}{
		"status":      "healthy",
		"transport":   "sse",
		"clients":     clientCount,
		"timestamp":   time.Now().Unix(),
	}
	
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(health)
}

// corsMiddleware adds CORS headers to responses
func (t *SSETransport) corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Set CORS headers
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "POST, GET, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, X-Session-ID, Last-Event-ID")
		w.Header().Set("Access-Control-Expose-Headers", "X-Session-ID")
		
		// Handle preflight requests
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		
		next.ServeHTTP(w, r)
	})
}

// sendSSEResponse sends a response to the SSE client
func (t *SSETransport) sendSSEResponse(sessionID string, resp *JSONRPCResponse) {
	t.queuesMu.RLock()
	queue, exists := t.messageQueues[sessionID]
	t.queuesMu.RUnlock()
	
	if exists {
		select {
		case queue <- resp:
			// Message queued successfully
		default:
			// Queue is full, drop message
			fmt.Printf("Warning: Message queue full for session %s\n", sessionID)
		}
	}
}

// BroadcastEvent sends an event to all connected clients
func (t *SSETransport) BroadcastEvent(event *SSEEvent) {
	t.clientsMu.RLock()
	defer t.clientsMu.RUnlock()
	
	for _, client := range t.clients {
		select {
		case client.EventChannel <- event:
			// Event sent
		default:
			// Client channel full, skip
		}
	}
}