package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"runtime/debug"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/JamesPrial/mcp-memory-core/internal/admin"
	"github.com/JamesPrial/mcp-memory-core/internal/knowledge"
	"github.com/JamesPrial/mcp-memory-core/internal/storage"
	"github.com/JamesPrial/mcp-memory-core/internal/transport"
	"github.com/JamesPrial/mcp-memory-core/pkg/config"
	"github.com/JamesPrial/mcp-memory-core/pkg/logging"
)


const version = "1.0.0" // TODO: Make this dynamic

// Server represents the MCP server
type Server struct {
	manager *knowledge.Manager
	logger  *slog.Logger
}

// NewServer creates a new MCP server
func NewServer(manager *knowledge.Manager, logger *slog.Logger) *Server {
	return &Server{
		manager: manager,
		logger:  logger,
	}
}

// validateRequest validates a JSON-RPC request and returns an error response if invalid
func (s *Server) validateRequest(req *transport.JSONRPCRequest) *transport.JSONRPCResponse {
	// Check if request is nil
	if req == nil {
		return &transport.JSONRPCResponse{
			JSONRPC: "2.0",
			ID:      nil,
			Error: &transport.JSONRPCError{
				Code:    -32600,
				Message: "Invalid Request",
				Data:    "Request cannot be null",
			},
		}
	}

	// Validate jsonrpc field
	if req.JSONRPC != "2.0" {
		return &transport.JSONRPCResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Error: &transport.JSONRPCError{
				Code:    -32600,
				Message: "Invalid Request",
				Data:    "Invalid or missing 'jsonrpc' field, must be '2.0'",
			},
		}
	}

	// Validate method field
	if req.Method == "" {
		return &transport.JSONRPCResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Error: &transport.JSONRPCError{
				Code:    -32600,
				Message: "Invalid Request",
				Data:    "Missing or empty 'method' field",
			},
		}
	}

	// ID field validation - it can be string, number, or null, but not missing entirely
	// In JSON-RPC 2.0, notifications don't have ID, but regular requests must have one
	if req.ID == nil {
		// This could be a notification, but for MCP we require IDs
		return &transport.JSONRPCResponse{
			JSONRPC: "2.0",
			ID:      nil,
			Error: &transport.JSONRPCError{
				Code:    -32600,
				Message: "Invalid Request",
				Data:    "Missing 'id' field - notifications are not supported",
			},
		}
	}

	// Check for invalid method patterns
	if len(req.Method) > 100 || strings.Contains(req.Method, "..") || strings.Contains(req.Method, "//") ||
		strings.Contains(req.Method, "\x00") || strings.Contains(req.Method, "\n") || strings.Contains(req.Method, "\t") {
		return &transport.JSONRPCResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Error: &transport.JSONRPCError{
				Code:    -32600,
				Message: "Invalid Request",
				Data:    fmt.Sprintf("Invalid method format: '%s'", req.Method),
			},
		}
	}

	return nil // Request is valid
}

// isValidToolName validates that a tool name is properly formatted
func isValidToolName(name string) bool {
	// Tool name must not be empty and should have reasonable length
	if name == "" || len(name) > 100 {
		return false
	}
	
	// Tool name should only contain lowercase alphanumeric characters, underscores, and hyphens
	// No special characters, unicode, spaces, etc.
	for _, r := range name {
		if !((r >= 'a' && r <= 'z') || 
			 (r >= 'A' && r <= 'Z') ||
			 (r >= '0' && r <= '9') || r == '_' || r == '-') {
			return false
		}
	}
	
	return true
}


// sanitizeError converts internal errors to user-safe messages
func (s *Server) sanitizeError(err error) string {
	if err == nil {
		return "Unknown error occurred"
	}

	errMsg := strings.ToLower(err.Error())
	
	// Replace internal error patterns with user-friendly messages
	if strings.Contains(errMsg, "database") || strings.Contains(errMsg, "sql") {
		return "Storage operation failed"
	}
	if strings.Contains(errMsg, "memory") || strings.Contains(errMsg, "allocation") {
		return "Memory operation failed"
	}
	if strings.Contains(errMsg, "permission") || strings.Contains(errMsg, "access") {
		return "Access denied"
	}
	if strings.Contains(errMsg, "connection") || strings.Contains(errMsg, "network") {
		return "Connection error"
	}
	if strings.Contains(errMsg, "timeout") {
		return "Operation timed out"
	}
	if strings.Contains(errMsg, "not found") {
		return "Requested resource not found"
	}
	if strings.Contains(errMsg, "invalid") || strings.Contains(errMsg, "malformed") {
		return "Invalid input provided"
	}

	// For any other errors, return a generic message
	return "Internal server error"
}

// HandleRequest processes a JSON-RPC request and returns a response
func (s *Server) HandleRequest(ctx context.Context, req *transport.JSONRPCRequest) *transport.JSONRPCResponse {
	// First, validate the request
	if validationErr := s.validateRequest(req); validationErr != nil {
		return validationErr
	}


	switch req.Method {
	case "tools/list":
		return s.handleToolsList(req)
	case "tools/call":
		return s.handleToolsCall(ctx, req)
	default:
		// This is -32601 Method not found (for valid format but unknown method)
		return &transport.JSONRPCResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Error: &transport.JSONRPCError{
				Code:    -32601,
				Message: "Method not found",
				Data:    fmt.Sprintf("Method '%s' is not supported", req.Method),
			},
		}
	}
}

// handleToolsList handles the tools/list method
func (s *Server) handleToolsList(req *transport.JSONRPCRequest) *transport.JSONRPCResponse {
	defer func() {
		if r := recover(); r != nil {
			if s.logger != nil {
				s.logger.Error("Panic in handleToolsList", 
					slog.Any("panic", r),
					slog.String("stack", string(debug.Stack())))
			} else {
				log.Printf("Panic in handleToolsList: %v", r)
			}
		}
	}()

	// Check if manager is nil
	if s.manager == nil {
		return &transport.JSONRPCResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Error: &transport.JSONRPCError{
				Code:    -32603,
				Message: "Internal error",
				Data:    "Server not properly initialized",
			},
		}
	}

	tools := s.manager.HandleListTools()
	
	return &transport.JSONRPCResponse{
		JSONRPC: "2.0",
		ID:      req.ID,
		Result: map[string]interface{}{
			"tools": tools,
		},
	}
}

// handleToolsCall handles the tools/call method
func (s *Server) handleToolsCall(ctx context.Context, req *transport.JSONRPCRequest) *transport.JSONRPCResponse {
	defer func() {
		if r := recover(); r != nil {
			if s.logger != nil {
				s.logger.Error("Panic in handleToolsCall", 
					slog.Any("panic", r),
					slog.String("stack", string(debug.Stack())))
			} else {
				log.Printf("Panic in handleToolsCall: %v", r)
			}
		}
	}()

	// Check if manager is nil
	if s.manager == nil {
		return &transport.JSONRPCResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Error: &transport.JSONRPCError{
				Code:    -32603,
				Message: "Internal error",
				Data:    "Server not properly initialized",
			},
		}
	}

	// Validate params exists
	if req.Params == nil {
		return &transport.JSONRPCResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Error: &transport.JSONRPCError{
				Code:    -32602,
				Message: "Invalid params",
				Data:    "Missing 'params' field for tools/call method",
			},
		}
	}

	// Extract tool name from params
	name, ok := req.Params["name"]
	if !ok {
		return &transport.JSONRPCResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Error: &transport.JSONRPCError{
				Code:    -32602,
				Message: "Invalid params",
				Data:    "Missing 'name' field in params",
			},
		}
	}

	toolName, ok := name.(string)
	if !ok {
		return &transport.JSONRPCResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Error: &transport.JSONRPCError{
				Code:    -32602,
				Message: "Invalid params",
				Data:    "Field 'name' must be a string",
			},
		}
	}

	if toolName == "" {
		return &transport.JSONRPCResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Error: &transport.JSONRPCError{
				Code:    -32602,
				Message: "Invalid params",
				Data:    "Field 'name' cannot be empty",
			},
		}
	}
	
	// Check arguments format first (before validating tool name)
	// This ensures we give consistent error messages for malformed arguments
	if args, exists := req.Params["arguments"]; exists {
		// nil arguments should be treated as empty map
		if args != nil {
			// Non-nil arguments must be a map
			if _, ok := args.(map[string]interface{}); !ok {
				// Non-nil, non-map arguments are invalid
				return &transport.JSONRPCResponse{
					JSONRPC: "2.0",
					ID:      req.ID,
					Error: &transport.JSONRPCError{
						Code:    -32602,
						Message: "Invalid params",
						Data:    "Field 'arguments' must be an object",
					},
				}
			}
		}
	}
	
	// Validate tool name format - should contain only alphanumeric, underscores
	// Tool names shouldn't have special characters, unicode, etc.
	if !isValidToolName(toolName) {
		return &transport.JSONRPCResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Error: &transport.JSONRPCError{
				Code:    -32602,
				Message: "Invalid params",
				Data:    "Field 'name' contains invalid characters or unknown tool",
			},
		}
	}
	
	// Check if the tool exists (basic validation for known patterns)
	if !strings.HasPrefix(toolName, "memory__") {
		// Tool doesn't match expected pattern
		return &transport.JSONRPCResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Error: &transport.JSONRPCError{
				Code:    -32602,
				Message: "Invalid params",
				Data:    "Unknown tool name",
			},
		}
	}

	// Extract arguments from params (we've already validated the format above)
	arguments := make(map[string]interface{})
	if args, exists := req.Params["arguments"]; exists {
		// nil arguments should be treated as empty map
		if args != nil {
			if argsMap, ok := args.(map[string]interface{}); ok {
				arguments = argsMap
			}
			// We already validated above, so this should always succeed
		}
	}

	// Call the tool
	result, err := s.manager.HandleCallTool(ctx, toolName, arguments)
	if err != nil {
		// Check if error needs sanitization
		errMsg := err.Error()
		needsSanitization := strings.Contains(strings.ToLower(errMsg), "password") ||
			strings.Contains(strings.ToLower(errMsg), "secret") ||
			strings.Contains(strings.ToLower(errMsg), "token") ||
			strings.Contains(strings.ToLower(errMsg), "key") ||
			strings.Contains(strings.ToLower(errMsg), "database connection")
		
		var message string
		if needsSanitization {
			message = s.sanitizeError(err)
		} else {
			message = fmt.Sprintf("Internal error: %s", err.Error())
		}
		
		return &transport.JSONRPCResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Error: &transport.JSONRPCError{
				Code:    -32603,
				Message: message,
				Data:    nil,
			},
		}
	}

	return &transport.JSONRPCResponse{
		JSONRPC: "2.0",
		ID:      req.ID,
		Result:  result,
	}
}


// convertLegacyLogConfig converts old LogLevel configuration to new logging config
func convertLegacyLogConfig(cfg *config.Settings) *logging.Config {
	logConfig := logging.DefaultConfig()
	
	// Convert legacy LogLevel if present and no new logging config
	if cfg.LogLevel != "" && cfg.Logging == nil {
		switch strings.ToLower(cfg.LogLevel) {
		case "debug":
			logConfig.Level = logging.LogLevelDebug
		case "info":
			logConfig.Level = logging.LogLevelInfo
		case "warn", "warning":
			logConfig.Level = logging.LogLevelWarn
		case "error":
			logConfig.Level = logging.LogLevelError
		default:
			logConfig.Level = logging.LogLevelInfo
		}
	}
	
	// Override with new logging config if present
	if cfg.Logging != nil {
		// Map config fields to logging config
		if cfg.Logging.Format != "" {
			if cfg.Logging.Format == "json" {
				logConfig.Format = logging.LogFormatJSON
			} else {
				logConfig.Format = logging.LogFormatText
			}
		}
		
		if cfg.Logging.Output != "" {
			switch cfg.Logging.Output {
			case "stdout":
				logConfig.Output = logging.LogOutputStdout
			case "stderr":
				logConfig.Output = logging.LogOutputStderr
			case "file":
				logConfig.Output = logging.LogOutputFile
				logConfig.FilePath = cfg.Logging.FilePath
			}
		}
		
		if cfg.Logging.BufferSize > 0 {
			logConfig.BufferSize = cfg.Logging.BufferSize
		}
		
		logConfig.AsyncLogging = cfg.Logging.AsyncLogging
		logConfig.EnableRequestID = cfg.Logging.EnableRequestID
		logConfig.EnableTracing = cfg.Logging.EnableTracing
		logConfig.EnableAudit = cfg.Logging.EnableAudit
		logConfig.AuditFilePath = cfg.Logging.AuditFilePath
		logConfig.EnableStackTrace = cfg.Logging.EnableStackTrace
		logConfig.EnableCaller = cfg.Logging.EnableCaller
		logConfig.PrettyPrint = cfg.Logging.PrettyPrint
		
		// Convert component levels
		if cfg.Logging.ComponentLevels != nil {
			logConfig.ComponentLevels = make(map[string]logging.LogLevel)
			for comp, level := range cfg.Logging.ComponentLevels {
				switch strings.ToLower(level) {
				case "debug":
					logConfig.ComponentLevels[comp] = logging.LogLevelDebug
				case "info":
					logConfig.ComponentLevels[comp] = logging.LogLevelInfo
				case "warn", "warning":
					logConfig.ComponentLevels[comp] = logging.LogLevelWarn
				case "error":
					logConfig.ComponentLevels[comp] = logging.LogLevelError
				}
			}
		}
		
		// Convert sampling settings
		if cfg.Logging.Sampling != nil {
			logConfig.Sampling.Enabled = cfg.Logging.Sampling.Enabled
			logConfig.Sampling.Rate = cfg.Logging.Sampling.Rate
			logConfig.Sampling.BurstSize = cfg.Logging.Sampling.BurstSize
			logConfig.Sampling.AlwaysErrors = cfg.Logging.Sampling.AlwaysErrors
		}
		
		// Convert masking settings
		if cfg.Logging.Masking != nil {
			logConfig.Masking.Enabled = cfg.Logging.Masking.Enabled
			logConfig.Masking.Fields = cfg.Logging.Masking.Fields
			logConfig.Masking.Patterns = cfg.Logging.Masking.Patterns
			logConfig.Masking.MaskEmails = cfg.Logging.Masking.MaskEmails
			logConfig.Masking.MaskPhoneNumbers = cfg.Logging.Masking.MaskPhoneNumbers
			logConfig.Masking.MaskCreditCards = cfg.Logging.Masking.MaskCreditCards
			logConfig.Masking.MaskSSN = cfg.Logging.Masking.MaskSSN
			logConfig.Masking.MaskAPIKeys = cfg.Logging.Masking.MaskAPIKeys
		}
		
		// Convert OTLP settings
		if cfg.Logging.OTLP != nil {
			logConfig.OTLP.Enabled = cfg.Logging.OTLP.Enabled
			logConfig.OTLP.Endpoint = cfg.Logging.OTLP.Endpoint
			logConfig.OTLP.Headers = cfg.Logging.OTLP.Headers
			logConfig.OTLP.Insecure = cfg.Logging.OTLP.Insecure
			logConfig.OTLP.Timeout = cfg.Logging.OTLP.Timeout
			logConfig.OTLP.BatchSize = cfg.Logging.OTLP.BatchSize
			logConfig.OTLP.QueueSize = cfg.Logging.OTLP.QueueSize
		}
	}
	
	// Apply environment variable overrides
	applyEnvOverrides(logConfig)
	
	return logConfig
}

// applyEnvOverrides applies environment variable overrides to logging config
func applyEnvOverrides(config *logging.Config) {
	if level := os.Getenv("MCP_LOG_LEVEL"); level != "" {
		switch strings.ToLower(level) {
		case "debug":
			config.Level = logging.LogLevelDebug
		case "info":
			config.Level = logging.LogLevelInfo
		case "warn", "warning":
			config.Level = logging.LogLevelWarn
		case "error":
			config.Level = logging.LogLevelError
		}
	}
	
	if format := os.Getenv("MCP_LOG_FORMAT"); format != "" {
		if format == "json" {
			config.Format = logging.LogFormatJSON
		} else {
			config.Format = logging.LogFormatText
		}
	}
	
	if output := os.Getenv("MCP_LOG_OUTPUT"); output != "" {
		switch output {
		case "stdout":
			config.Output = logging.LogOutputStdout
		case "stderr":
			config.Output = logging.LogOutputStderr
		case "file":
			config.Output = logging.LogOutputFile
			if path := os.Getenv("MCP_LOG_FILE_PATH"); path != "" {
				config.FilePath = path
			}
		}
	}
	
	if asyncStr := os.Getenv("MCP_LOG_ASYNC"); asyncStr != "" {
		if async, err := strconv.ParseBool(asyncStr); err == nil {
			config.AsyncLogging = async
		}
	}
	
	if auditStr := os.Getenv("MCP_LOG_AUDIT"); auditStr != "" {
		if audit, err := strconv.ParseBool(auditStr); err == nil {
			config.EnableAudit = audit
		}
	}
}

// maskSensitiveConfig creates a copy of config with sensitive data masked
func maskSensitiveConfig(cfg *config.Settings) map[string]interface{} {
	masked := make(map[string]interface{})
	masked["storageType"] = cfg.StorageType
	masked["transportType"] = cfg.TransportType
	masked["httpPort"] = cfg.HTTPPort
	masked["logLevel"] = cfg.LogLevel
	
	// Mask storage path if it contains sensitive info
	if cfg.StoragePath != "" {
		if strings.Contains(cfg.StoragePath, "password") || strings.Contains(cfg.StoragePath, "secret") {
			masked["storagePath"] = "[MASKED]"
		} else {
			masked["storagePath"] = cfg.StoragePath
		}
	}
	
	// Include transport settings but mask sensitive data
	if cfg.Transport.Host != "" {
		masked["transport"] = map[string]interface{}{
			"host": cfg.Transport.Host,
			"port": cfg.Transport.Port,
			"readTimeout": cfg.Transport.ReadTimeout,
			"writeTimeout": cfg.Transport.WriteTimeout,
			"maxConnections": cfg.Transport.MaxConnections,
			"enableCORS": cfg.Transport.EnableCORS,
		}
	}
	
	return masked
}

// setupSignalHandling sets up signal handlers for graceful shutdown and log level changes
func setupSignalHandling(ctx context.Context, cancel context.CancelFunc, logger *slog.Logger) {
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM, syscall.SIGUSR1, syscall.SIGUSR2)
	
	// Track current debug state for toggle functionality
	debugEnabled := false
	
	go func() {
		for {
			select {
			case sig := <-sigChan:
				switch sig {
				case os.Interrupt, syscall.SIGTERM:
					logger.Info("Received shutdown signal", slog.String("signal", sig.String()))
					cancel()
					return
				case syscall.SIGUSR1:
					// Toggle debug logging for all components
					if debugEnabled {
						logger.Info("SIGUSR1: Disabling debug logging")
						logging.UpdateGlobalLevel("main", logging.LogLevelInfo)
						logging.UpdateGlobalLevel("server", logging.LogLevelInfo)
						logging.UpdateGlobalLevel("storage", logging.LogLevelInfo)
						logging.UpdateGlobalLevel("transport", logging.LogLevelInfo)
						logging.UpdateGlobalLevel("knowledge", logging.LogLevelInfo)
						debugEnabled = false
					} else {
						logger.Info("SIGUSR1: Enabling debug logging")
						logging.UpdateGlobalLevel("main", logging.LogLevelDebug)
						logging.UpdateGlobalLevel("server", logging.LogLevelDebug)
						logging.UpdateGlobalLevel("storage", logging.LogLevelDebug)
						logging.UpdateGlobalLevel("transport", logging.LogLevelDebug)
						logging.UpdateGlobalLevel("knowledge", logging.LogLevelDebug)
						debugEnabled = true
					}
				case syscall.SIGUSR2:
					// Rotate logs if using file output (placeholder)
					logger.Info("SIGUSR2: Log rotation requested (not implemented)")
				}
			case <-ctx.Done():
				return
			}
		}
	}()
}

func main() {
	startTime := time.Now()
	
	// Parse command-line flags
	var configPath string
	flag.StringVar(&configPath, "config", "config.yaml", "Path to configuration file")
	flag.Parse()

	// Load configuration
	cfg, err := config.Load(configPath)
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	// Initialize logging system first
	logConfig := convertLegacyLogConfig(cfg)
	if err := logging.Initialize(logConfig); err != nil {
		log.Fatalf("Failed to initialize logging: %v", err)
	}
	defer func() {
		if err := logging.Shutdown(); err != nil {
			log.Printf("Error shutting down logging: %v", err)
		}
	}()
	
	// Get main logger
	logger := logging.GetGlobalLogger("main")
	logger.Info("Starting MCP Memory Server", 
		slog.String("version", version),
		slog.Any("config", maskSensitiveConfig(cfg)),
		slog.Duration("config_load_time", time.Since(startTime)))
	
	// Set up panic recovery
	defer func() {
		if r := recover(); r != nil {
			logger.Error("Server panic", 
				slog.Any("panic", r),
				slog.String("stack", string(debug.Stack())))
			os.Exit(1)
		}
	}()

	// Initialize storage backend
	storageStart := time.Now()
	backend, err := storage.NewBackend(cfg)
	if err != nil {
		logger.Error("Failed to create storage backend", slog.Any("error", err))
		os.Exit(1)
	}
	defer func() {
		if err := backend.Close(); err != nil {
			logger.Error("Error closing storage backend", slog.Any("error", err))
		}
	}()
	logger.Info("Storage backend initialized", 
		slog.String("type", cfg.StorageType),
		slog.Duration("init_time", time.Since(storageStart)))

	// Create knowledge manager
	knowledgeStart := time.Now()
	manager := knowledge.NewManager(backend)
	logger.Info("Knowledge manager initialized", 
		slog.Duration("init_time", time.Since(knowledgeStart)))

	// Create server with logger
	serverLogger := logging.GetGlobalLogger("server")
	server := NewServer(manager, serverLogger)
	
	// Create transport based on configuration
	transportStart := time.Now()
	factory := transport.NewFactory()
	trans, err := factory.CreateTransport(cfg)
	if err != nil {
		logger.Error("Failed to create transport", slog.Any("error", err))
		os.Exit(1)
	}
	logger.Info("Transport initialized", 
		slog.String("type", cfg.TransportType),
		slog.String("name", trans.Name()),
		slog.Duration("init_time", time.Since(transportStart)))
	
	// Set up context with signal handling
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	
	// Set up signal handlers
	setupSignalHandling(ctx, cancel, logger)
	
	// Start admin server if HTTP transport is enabled (reuse the port + 1000)
	var adminServer *admin.AdminServer
	if cfg.TransportType == "http" && cfg.Transport.Port > 0 {
		adminPort := cfg.Transport.Port + 1000
		adminServer = admin.NewAdminServer()
		
		go func() {
			logger.Info("Starting admin server", slog.Int("port", adminPort))
			if err := adminServer.StartServer(adminPort); err != nil && err != http.ErrServerClosed {
				logger.Error("Admin server error", slog.Any("error", err))
			}
		}()
	}
	
	// Start metrics server if enabled
	metricsCollector := logging.GetGlobalMetricsCollector()
	if metricsCollector != nil {
		go func() {
			logger.Info("Starting metrics server")
			if err := metricsCollector.StartMetricsServer(); err != nil && err != http.ErrServerClosed {
				logger.Error("Metrics server error", slog.Any("error", err))
			}
		}()
	}
	
	// Log successful startup
	totalStartTime := time.Since(startTime)
	logger.Info("Server initialization completed", 
		slog.Duration("total_startup_time", totalStartTime))
	
	// Start the transport with request handler
	logger.Info("Starting transport", slog.String("transport", trans.Name()))
	if err := trans.Start(ctx, server.HandleRequest); err != nil {
		if err != context.Canceled {
			logger.Error("Transport error", slog.Any("error", err))
			os.Exit(1)
		}
	}
	
	logger.Info("Server stopped gracefully")
}