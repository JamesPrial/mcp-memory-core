package transport

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"github.com/JamesPrial/mcp-memory-core/pkg/config"
	"github.com/JamesPrial/mcp-memory-core/pkg/logging"
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
	logger         *slog.Logger
	interceptor    *logging.RequestInterceptor
	clients        map[string]*SSEClient
	clientsMu      sync.RWMutex
	messageQueues  map[string]chan *JSONRPCResponse
	queuesMu       sync.RWMutex
	mu             sync.RWMutex
}

// NewSSETransport creates a new SSE transport
func NewSSETransport(cfg *config.TransportSettings) *SSETransport {
	logger := logging.GetGlobalLogger("transport.sse")
	return &SSETransport{
		config:         cfg,
		sessionManager: NewSessionManager(30 * time.Minute),
		logger:         logger,
		interceptor:    logging.NewRequestInterceptor(logger),
		clients:        make(map[string]*SSEClient),
		messageQueues:  make(map[string]chan *JSONRPCResponse),
	}
}

// Start begins listening for SSE connections
func (t *SSETransport) Start(ctx context.Context, handler RequestHandler) error {
	t.mu.Lock()
	t.handler = handler
	t.mu.Unlock()
	
	// Log transport startup
	t.logger.InfoContext(ctx, "SSE transport starting",
		slog.String("transport", "sse"),
		slog.String("host", t.config.Host),
		slog.Int("port", t.config.Port),
	)
	
	mux := http.NewServeMux()
	
	// Main RPC endpoint (accepts POST requests)
	mux.HandleFunc("/rpc", t.handleRPC)
	
	// SSE events endpoint
	mux.HandleFunc("/events", t.handleSSE)
	
	// Health check endpoint
	mux.HandleFunc("/health", t.handleHealth)
	
	// Apply request logging middleware
	var httpHandler http.Handler = mux
	httpHandler = t.interceptor.HTTPMiddleware(httpHandler)
	
	// CORS middleware
	if t.config.EnableCORS {
		httpHandler = t.corsMiddleware(httpHandler)
	}
	
	// Create HTTP server
	addr := fmt.Sprintf("%s:%d", t.config.Host, t.config.Port)
	t.mu.Lock()
	t.server = &http.Server{
		Addr:         addr,
		Handler:      httpHandler,
		ReadTimeout:  time.Duration(t.config.ReadTimeout) * time.Second,
		WriteTimeout: time.Duration(t.config.WriteTimeout) * time.Second,
	}
	server := t.server
	t.mu.Unlock()
	
	t.logger.InfoContext(ctx, "SSE server starting",
		slog.String("address", addr),
	)
	
	// Start server in goroutine
	errChan := make(chan error, 1)
	go func() {
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			t.logger.ErrorContext(ctx, "SSE server error",
				slog.String("error", err.Error()),
			)
			errChan <- err
		}
	}()
	
	// Wait for context cancellation or server error
	select {
	case <-ctx.Done():
		t.logger.InfoContext(ctx, "SSE transport context cancelled")
		return t.Stop(context.Background())
	case err := <-errChan:
		return err
	}
}

// Stop gracefully shuts down the SSE server
func (t *SSETransport) Stop(ctx context.Context) error {
	t.logger.InfoContext(ctx, "SSE transport stopping")
	
	// Close all client connections
	t.clientsMu.Lock()
	clientCount := len(t.clients)
	for _, client := range t.clients {
		close(client.Done)
	}
	t.clientsMu.Unlock()
	
	t.logger.InfoContext(ctx, "Closed SSE client connections",
		slog.Int("client_count", clientCount),
	)
	
	t.mu.RLock()
	server := t.server
	t.mu.RUnlock()
	
	if server != nil {
		t.sessionManager.Stop()
		err := server.Shutdown(ctx)
		if err != nil {
			t.logger.ErrorContext(ctx, "Error during SSE server shutdown",
				slog.String("error", err.Error()),
			)
		} else {
			t.logger.InfoContext(ctx, "SSE transport stopped successfully")
		}
		return err
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
		respErr := CreateFallbackErrorResponse(nil, "Method not allowed")
		t.sendJSONErrorResponse(w, respErr, http.StatusMethodNotAllowed)
		return
	}
	
	// Check content type
	contentType := r.Header.Get("Content-Type")
	if contentType != "application/json" && contentType != "application/json-rpc" {
		respErr := CreateFallbackErrorResponse(nil, "Content-Type must be application/json or application/json-rpc")
		t.sendJSONErrorResponse(w, respErr, http.StatusBadRequest)
		return
	}
	
	// Get session ID
	sessionID := r.Header.Get("X-Session-ID")
	if sessionID == "" {
		respErr := CreateFallbackErrorResponse(nil, "X-Session-ID header required")
		t.sendJSONErrorResponse(w, respErr, http.StatusBadRequest)
		return
	}
	
	// Check if session exists
	if _, exists := t.sessionManager.GetSession(sessionID); !exists {
		respErr := CreateFallbackErrorResponse(nil, "Invalid or expired session")
		t.sendJSONErrorResponse(w, respErr, http.StatusUnauthorized)
		return
	}
	
	// Read request body
	body, err := io.ReadAll(io.LimitReader(r.Body, 10*1024*1024)) // 10MB limit
	if err != nil {
		respErr := CreateFallbackErrorResponse(nil, "Failed to read request body")
		t.sendJSONErrorResponse(w, respErr, http.StatusBadRequest)
		return
	}
	defer r.Body.Close()
	
	// Parse JSON-RPC request
	var req JSONRPCRequest
	if err := json.Unmarshal(body, &req); err != nil {
		respErr := NewParseError()
		t.sendSSEResponse(sessionID, respErr)
		w.WriteHeader(http.StatusAccepted)
		return
	}
	
	// Handle the request
	ctx := r.Context()
	t.mu.RLock()
	handler := t.handler
	t.mu.RUnlock()
	resp := handler(ctx, &req)
	
	// Send response via SSE
	t.sendSSEResponse(sessionID, resp)
	
	// Return 202 Accepted to indicate the request was received
	w.WriteHeader(http.StatusAccepted)
	w.Write([]byte(`{"status":"accepted"}`))
}

// handleSSE handles SSE connections
func (t *SSETransport) handleSSE(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	
	t.logger.InfoContext(ctx, "New SSE connection request",
		slog.String("remote_addr", r.RemoteAddr),
		slog.String("user_agent", r.UserAgent()),
	)
	
	// Check if SSE is supported
	flusher, ok := w.(http.Flusher)
	if !ok {
		t.logger.ErrorContext(ctx, "SSE not supported by response writer")
		respErr := CreateFallbackErrorResponse(nil, "SSE not supported")
		t.sendJSONErrorResponse(w, respErr, http.StatusInternalServerError)
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
			t.logger.ErrorContext(ctx, "Failed to create SSE session",
				slog.String("error", err.Error()),
			)
			// Send error as SSE event since headers are already set
			errorResp := ToJSONRPCResponse(nil, err)
			t.sendErrorEvent(w, flusher, errorResp)
			return
		}
		sessionID = session.ID
		
		t.logger.InfoContext(ctx, "Created new SSE session",
			slog.String("session_id", sessionID),
		)
		
		// Send session ID as first event
		fmt.Fprintf(w, "event: session\ndata: {\"sessionId\":\"%s\"}\n\n", sessionID)
		flusher.Flush()
	} else {
		if _, exists := t.sessionManager.GetSession(sessionID); !exists {
			t.logger.WarnContext(ctx, "SSE session expired, creating new one",
				slog.String("old_session_id", sessionID),
			)
			// Session expired, create new one
			session, err := t.sessionManager.CreateSession("sse")
			if err != nil {
				t.logger.ErrorContext(ctx, "Failed to create replacement SSE session",
					slog.String("error", err.Error()),
				)
				// Send error as SSE event since headers are already set
				errorResp := ToJSONRPCResponse(nil, err)
				t.sendErrorEvent(w, flusher, errorResp)
				return
			}
			sessionID = session.ID
			t.logger.InfoContext(ctx, "Created replacement SSE session",
				slog.String("session_id", sessionID),
			)
			fmt.Fprintf(w, "event: session\ndata: {\"sessionId\":\"%s\"}\n\n", sessionID)
			flusher.Flush()
		} else {
			t.logger.DebugContext(ctx, "Using existing SSE session",
				slog.String("session_id", sessionID),
			)
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
	clientCount := len(t.clients)
	t.clientsMu.Unlock()
	
	t.logger.InfoContext(ctx, "SSE client connected",
		slog.String("session_id", sessionID),
		slog.Int("total_clients", clientCount),
	)
	
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
		remainingClients := len(t.clients)
		t.clientsMu.Unlock()
		close(client.EventChannel)
		
		t.logger.InfoContext(ctx, "SSE client disconnected",
			slog.String("session_id", sessionID),
			slog.Int("remaining_clients", remainingClients),
		)
	}()
	
	// Send initial connection event
	fmt.Fprintf(w, "event: connected\ndata: {\"status\":\"connected\"}\n\n")
	flusher.Flush()
	
	// Handle reconnection with Last-Event-ID
	if lastEventID != "" {
		t.logger.InfoContext(ctx, "SSE client reconnection",
			slog.String("session_id", sessionID),
			slog.String("last_event_id", lastEventID),
		)
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

// sendJSONErrorResponse sends a JSON error response with proper status code
func (t *SSETransport) sendJSONErrorResponse(w http.ResponseWriter, resp *JSONRPCResponse, statusCode int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		// Critical failure - try fallback
		fallbackResp := CreateFallbackErrorResponse(resp.ID, "Failed to encode error response")
		if fallbackBytes, fallbackErr := json.Marshal(fallbackResp); fallbackErr == nil {
			w.Write(fallbackBytes)
		}
	}
}

// sendErrorEvent sends an error as an SSE event when headers are already set
func (t *SSETransport) sendErrorEvent(w http.ResponseWriter, flusher http.Flusher, errorResp *JSONRPCResponse) {
	data, err := json.Marshal(errorResp)
	if err != nil {
		// Fallback error event
		fmt.Fprintf(w, "event: error\ndata: {\"error\":\"Failed to serialize error\"}\n\n")
	} else {
		fmt.Fprintf(w, "event: error\ndata: %s\n\n", string(data))
	}
	flusher.Flush()
}