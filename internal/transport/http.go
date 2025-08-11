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
	"github.com/JamesPrial/mcp-memory-core/pkg/errors"
	"github.com/JamesPrial/mcp-memory-core/pkg/logging"
)

// HTTPTransport implements the Transport interface for HTTP communication
type HTTPTransport struct {
	config         *config.TransportSettings
	server         *http.Server
	handler        RequestHandler
	sessionManager *SessionManager
	logger         *slog.Logger
	interceptor    *logging.RequestInterceptor
	mu             sync.RWMutex
}

// NewHTTPTransport creates a new HTTP transport
func NewHTTPTransport(cfg *config.TransportSettings) *HTTPTransport {
	logger := logging.GetGlobalLogger("transport.http")
	return &HTTPTransport{
		config:         cfg,
		sessionManager: NewSessionManager(30 * time.Minute),
		logger:         logger,
		interceptor:    logging.NewRequestInterceptor(logger),
	}
}

// Start begins listening for HTTP requests
func (t *HTTPTransport) Start(ctx context.Context, handler RequestHandler) error {
	t.handler = handler
	
	// Log transport startup
	t.logger.InfoContext(ctx, "HTTP transport starting",
		slog.String("transport", "http"),
		slog.String("host", t.config.Host),
		slog.Int("port", t.config.Port),
	)
	
	mux := http.NewServeMux()
	
	// Main RPC endpoint
	mux.HandleFunc("/rpc", t.handleRPC)
	
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
	t.server = &http.Server{
		Addr:         addr,
		Handler:      httpHandler,
		ReadTimeout:  time.Duration(t.config.ReadTimeout) * time.Second,
		WriteTimeout: time.Duration(t.config.WriteTimeout) * time.Second,
	}
	
	t.logger.InfoContext(ctx, "HTTP server starting",
		slog.String("address", addr),
	)
	
	// Start server in goroutine
	errChan := make(chan error, 1)
	go func() {
		if err := t.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			t.logger.ErrorContext(ctx, "HTTP server error",
				slog.String("error", err.Error()),
			)
			errChan <- err
		}
	}()
	
	// Wait for context cancellation or server error
	select {
	case <-ctx.Done():
		t.logger.InfoContext(ctx, "HTTP transport context cancelled")
		return t.Stop(context.Background())
	case err := <-errChan:
		return err
	}
}

// Stop gracefully shuts down the HTTP server
func (t *HTTPTransport) Stop(ctx context.Context) error {
	t.logger.InfoContext(ctx, "HTTP transport stopping")
	
	if t.server != nil {
		t.sessionManager.Stop()
		err := t.server.Shutdown(ctx)
		if err != nil {
			t.logger.ErrorContext(ctx, "Error during HTTP server shutdown",
				slog.String("error", err.Error()),
			)
		} else {
			t.logger.InfoContext(ctx, "HTTP transport stopped successfully")
		}
		return err
	}
	return nil
}

// Name returns the name of the transport
func (t *HTTPTransport) Name() string {
	return "http"
}

// handleRPC handles JSON-RPC requests
func (t *HTTPTransport) handleRPC(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	
	// Only accept POST requests
	if r.Method != http.MethodPost {
		t.logger.WarnContext(ctx, "Invalid HTTP method for RPC",
			slog.String("method", r.Method),
			slog.String("path", r.URL.Path),
		)
		respErr := CreateFallbackErrorResponse(nil, "Method not allowed")
		t.sendJSONResponseWithStatus(w, respErr, http.StatusMethodNotAllowed)
		return
	}
	
	// Check content type
	contentType := r.Header.Get("Content-Type")
	if contentType != "application/json" && contentType != "application/json-rpc" {
		t.logger.WarnContext(ctx, "Invalid content type for RPC",
			slog.String("content_type", contentType),
			slog.String("path", r.URL.Path),
		)
		respErr := CreateFallbackErrorResponse(nil, "Content-Type must be application/json or application/json-rpc")
		t.sendJSONResponseWithStatus(w, respErr, http.StatusBadRequest)
		return
	}
	
	// Read request body
	body, err := io.ReadAll(io.LimitReader(r.Body, 10*1024*1024)) // 10MB limit
	if err != nil {
		t.logger.ErrorContext(ctx, "Failed to read request body",
			slog.String("error", err.Error()),
		)
		respErr := CreateFallbackErrorResponse(nil, "Failed to read request body")
		t.sendJSONResponseWithStatus(w, respErr, http.StatusBadRequest)
		return
	}
	defer r.Body.Close()
	
	// Parse JSON-RPC request
	var req JSONRPCRequest
	if err := json.Unmarshal(body, &req); err != nil {
		bodyPreview := string(body)
		if len(bodyPreview) > 200 {
			bodyPreview = bodyPreview[:200] + "..."
		}
		t.logger.WarnContext(ctx, "Failed to parse JSON-RPC request",
			slog.String("error", err.Error()),
			slog.String("body_preview", bodyPreview),
		)
		respErr := NewParseError()
		t.sendJSONResponseWithStatus(w, respErr, http.StatusOK) // JSON-RPC parse errors should return HTTP 200
		return
	}
	
	// Get or create session
	sessionID := r.Header.Get("X-Session-ID")
	if sessionID == "" {
		session, err := t.sessionManager.CreateSession("http")
		if err != nil {
			t.logger.ErrorContext(ctx, "Failed to create new session",
				slog.String("error", err.Error()),
			)
			respErr := ToJSONRPCResponse(req.ID, err)
			statusCode := ToHTTPStatusCode(err)
			t.sendJSONResponseWithStatus(w, respErr, statusCode)
			return
		}
		sessionID = session.ID
		w.Header().Set("X-Session-ID", sessionID)
		t.logger.DebugContext(ctx, "Created new session",
			slog.String("session_id", sessionID),
		)
	} else {
		if _, exists := t.sessionManager.GetSession(sessionID); !exists {
			t.logger.WarnContext(ctx, "Session expired or invalid, creating new one",
				slog.String("old_session_id", sessionID),
			)
			// Session expired or invalid, create new one
			session, err := t.sessionManager.CreateSession("http")
			if err != nil {
				t.logger.ErrorContext(ctx, "Failed to create replacement session",
					slog.String("error", err.Error()),
				)
				respErr := ToJSONRPCResponse(req.ID, err)
				statusCode := ToHTTPStatusCode(err)
				t.sendJSONResponseWithStatus(w, respErr, statusCode)
				return
			}
			sessionID = session.ID
			w.Header().Set("X-Session-ID", sessionID)
			t.logger.DebugContext(ctx, "Created replacement session",
				slog.String("session_id", sessionID),
			)
		} else {
			t.logger.DebugContext(ctx, "Using existing session",
				slog.String("session_id", sessionID),
			)
		}
	}
	
	// Handle the request with timing
	t.logger.InfoContext(ctx, "Processing JSON-RPC request",
		slog.String("method", req.Method),
		slog.Any("id", req.ID),
		slog.String("session_id", sessionID),
	)
	
	startTime := time.Now()
	resp := t.handler(ctx, &req)
	duration := time.Since(startTime)
	
	// Log response
	if resp.Error != nil {
		t.logger.WarnContext(ctx, "JSON-RPC request completed with error",
			slog.String("method", req.Method),
			slog.Any("id", req.ID),
			slog.String("session_id", sessionID),
			slog.Duration("duration", duration),
			slog.String("error", resp.Error.Message),
		)
	} else {
		t.logger.InfoContext(ctx, "JSON-RPC request completed successfully",
			slog.String("method", req.Method),
			slog.Any("id", req.ID),
			slog.String("session_id", sessionID),
			slog.Duration("duration", duration),
		)
	}
	
	// Determine appropriate HTTP status code based on response
	var statusCode int
	if resp.Error != nil {
		// Extract error code from response data if available
		if resp.Error.Data != nil {
			if dataMap, ok := resp.Error.Data.(map[string]interface{}); ok {
				if errorCodeStr, ok := dataMap["error_code"].(string); ok {
					// Create a mock AppError to use our status mapping
					mockErr := &errors.AppError{Code: errors.ErrorCode(errorCodeStr)}
					statusCode = ToHTTPStatusCode(mockErr)
				} else {
					statusCode = http.StatusInternalServerError
				}
			} else {
				statusCode = http.StatusInternalServerError
			}
		} else {
			statusCode = http.StatusInternalServerError
		}
	} else {
		statusCode = http.StatusOK
	}
	
	// Send response
	t.sendJSONResponseWithStatus(w, resp, statusCode)
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

// sendJSONResponse sends a JSON response with default OK status
func (t *HTTPTransport) sendJSONResponse(w http.ResponseWriter, resp *JSONRPCResponse, statusCode int) {
	t.sendJSONResponseWithStatus(w, resp, statusCode)
}

// sendJSONResponseWithStatus sends a JSON response with proper error handling
func (t *HTTPTransport) sendJSONResponseWithStatus(w http.ResponseWriter, resp *JSONRPCResponse, statusCode int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		// Critical failure - try to send a fallback response if headers not sent yet
		// Since headers are already sent, we can only log the error
		fmt.Printf("Error encoding response: %v\n", err)
		
		// Create fallback response in case encoding fails
		fallbackResp := CreateFallbackErrorResponse(resp.ID, "Failed to encode response")
		if fallbackBytes, fallbackErr := json.Marshal(fallbackResp); fallbackErr == nil {
			// Try to write fallback, but headers are already sent so this might not work
			w.Write(fallbackBytes)
		}
	}
}