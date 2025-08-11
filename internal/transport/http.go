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

// HTTPTransport implements the Transport interface for HTTP communication
type HTTPTransport struct {
	config         *config.TransportSettings
	server         *http.Server
	handler        RequestHandler
	sessionManager *SessionManager
	mu             sync.RWMutex
}

// NewHTTPTransport creates a new HTTP transport
func NewHTTPTransport(cfg *config.TransportSettings) *HTTPTransport {
	return &HTTPTransport{
		config:         cfg,
		sessionManager: NewSessionManager(30 * time.Minute),
	}
}

// Start begins listening for HTTP requests
func (t *HTTPTransport) Start(ctx context.Context, handler RequestHandler) error {
	t.handler = handler
	
	mux := http.NewServeMux()
	
	// Main RPC endpoint
	mux.HandleFunc("/rpc", t.handleRPC)
	
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

// Stop gracefully shuts down the HTTP server
func (t *HTTPTransport) Stop(ctx context.Context) error {
	if t.server != nil {
		t.sessionManager.Stop()
		return t.server.Shutdown(ctx)
	}
	return nil
}

// Name returns the name of the transport
func (t *HTTPTransport) Name() string {
	return "http"
}

// handleRPC handles JSON-RPC requests
func (t *HTTPTransport) handleRPC(w http.ResponseWriter, r *http.Request) {
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
		t.sendJSONResponse(w, NewParseError(), http.StatusOK)
		return
	}
	
	// Get or create session
	sessionID := r.Header.Get("X-Session-ID")
	if sessionID == "" {
		session, err := t.sessionManager.CreateSession("http")
		if err != nil {
			t.sendJSONResponse(w, NewInternalError(req.ID, "Failed to create session"), http.StatusInternalServerError)
			return
		}
		sessionID = session.ID
		w.Header().Set("X-Session-ID", sessionID)
	} else {
		if _, exists := t.sessionManager.GetSession(sessionID); !exists {
			// Session expired or invalid, create new one
			session, err := t.sessionManager.CreateSession("http")
			if err != nil {
				t.sendJSONResponse(w, NewInternalError(req.ID, "Failed to create session"), http.StatusInternalServerError)
				return
			}
			sessionID = session.ID
			w.Header().Set("X-Session-ID", sessionID)
		}
	}
	
	// Handle the request
	ctx := r.Context()
	resp := t.handler(ctx, &req)
	
	// Send response
	t.sendJSONResponse(w, resp, http.StatusOK)
}

// handleHealth handles health check requests
func (t *HTTPTransport) handleHealth(w http.ResponseWriter, r *http.Request) {
	health := map[string]interface{}{
		"status":  "healthy",
		"transport": "http",
		"timestamp": time.Now().Unix(),
	}
	
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(health)
}

// corsMiddleware adds CORS headers to responses
func (t *HTTPTransport) corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Set CORS headers
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "POST, GET, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, X-Session-ID")
		w.Header().Set("Access-Control-Expose-Headers", "X-Session-ID")
		
		// Handle preflight requests
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		
		next.ServeHTTP(w, r)
	})
}

// sendJSONResponse sends a JSON response
func (t *HTTPTransport) sendJSONResponse(w http.ResponseWriter, resp *JSONRPCResponse, statusCode int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		// Log error but response headers are already sent
		fmt.Printf("Error encoding response: %v\n", err)
	}
}