package admin

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strings"

	"github.com/JamesPrial/mcp-memory-core/pkg/logging"
)

// AdminServer provides administrative endpoints for runtime configuration
type AdminServer struct {
	logger *slog.Logger
	mux    *http.ServeMux
}

// NewAdminServer creates a new admin server
func NewAdminServer() *AdminServer {
	admin := &AdminServer{
		logger: logging.GetGlobalLogger("admin"),
		mux:    http.NewServeMux(),
	}
	
	admin.setupRoutes()
	return admin
}

// setupRoutes configures the admin endpoints
func (a *AdminServer) setupRoutes() {
	a.mux.HandleFunc("/health", a.handleHealth)
	a.mux.HandleFunc("/log-level", a.handleLogLevel)
	a.mux.HandleFunc("/log-levels", a.handleLogLevels)
	a.mux.HandleFunc("/metrics", a.handleMetrics)
}

// ServeHTTP implements http.Handler
func (a *AdminServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	a.mux.ServeHTTP(w, r)
}

// handleHealth returns server health status
func (a *AdminServer) handleHealth(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	
	response := map[string]interface{}{
		"status": "healthy",
		"server": "mcp-memory-core",
	}
	
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// handleLogLevel handles GET/POST requests to view/update log levels
func (a *AdminServer) handleLogLevel(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		a.getLogLevel(w, r)
	case http.MethodPost:
		a.setLogLevel(w, r)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// LogLevelRequest represents a log level change request
type LogLevelRequest struct {
	Component string `json:"component"`
	Level     string `json:"level"`
}

// LogLevelResponse represents a log level response
type LogLevelResponse struct {
	Component string `json:"component"`
	Level     string `json:"level"`
	Success   bool   `json:"success"`
	Message   string `json:"message,omitempty"`
}

// getLogLevel returns current log levels
func (a *AdminServer) getLogLevel(w http.ResponseWriter, r *http.Request) {
	component := r.URL.Query().Get("component")
	if component == "" {
		component = "default"
	}
	
	// For now, we'll return a placeholder since we can't easily query current levels
	// In a full implementation, this would query the logging factory
	response := LogLevelResponse{
		Component: component,
		Level:     "info", // Default assumption
		Success:   true,
	}
	
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// setLogLevel updates log level for a component
func (a *AdminServer) setLogLevel(w http.ResponseWriter, r *http.Request) {
	var req LogLevelRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response := LogLevelResponse{
			Success: false,
			Message: fmt.Sprintf("Invalid JSON: %v", err),
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(response)
		return
	}
	
	// Validate level
	validLevels := map[string]logging.LogLevel{
		"debug": logging.LogLevelDebug,
		"info":  logging.LogLevelInfo,
		"warn":  logging.LogLevelWarn,
		"error": logging.LogLevelError,
	}
	
	level, valid := validLevels[strings.ToLower(req.Level)]
	if !valid {
		response := LogLevelResponse{
			Component: req.Component,
			Success:   false,
			Message:   fmt.Sprintf("Invalid log level '%s'. Must be one of: debug, info, warn, error", req.Level),
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(response)
		return
	}
	
	// Update the log level
	component := req.Component
	if component == "" {
		component = "default"
	}
	
	logging.UpdateGlobalLevel(component, level)
	
	a.logger.Info("Log level updated", 
		slog.String("component", component),
		slog.String("level", req.Level))
	
	response := LogLevelResponse{
		Component: component,
		Level:     req.Level,
		Success:   true,
		Message:   fmt.Sprintf("Log level for component '%s' updated to '%s'", component, req.Level),
	}
	
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// handleLogLevels returns all component log levels (placeholder)
func (a *AdminServer) handleLogLevels(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	
	// Placeholder response - in a full implementation this would query the actual levels
	response := map[string]interface{}{
		"levels": map[string]string{
			"main":      "info",
			"server":    "info",
			"storage":   "info",
			"transport": "info",
			"knowledge": "info",
		},
	}
	
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// handleMetrics returns basic server metrics (placeholder)
func (a *AdminServer) handleMetrics(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	
	// Check if metrics collector is available
	metricsCollector := logging.GetGlobalMetricsCollector()
	if metricsCollector != nil {
		// Delegate to the metrics collector's HTTP handler
		metricsCollector.GetHTTPHandler().ServeHTTP(w, r)
		return
	}
	
	// Fallback response if no metrics collector
	response := map[string]interface{}{
		"status": "metrics not enabled",
	}
	
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// StartServer starts the admin server on the specified port
func (a *AdminServer) StartServer(port int) error {
	addr := fmt.Sprintf(":%d", port)
	a.logger.Info("Starting admin server", slog.String("address", addr))
	
	server := &http.Server{
		Addr:    addr,
		Handler: a,
	}
	
	return server.ListenAndServe()
}