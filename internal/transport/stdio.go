package transport

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"os"
	"strings"
	"time"

	"github.com/JamesPrial/mcp-memory-core/pkg/errors"
	"github.com/JamesPrial/mcp-memory-core/pkg/logging"
)

// StdioTransport implements the Transport interface for stdio communication
type StdioTransport struct {
	scanner     *bufio.Scanner
	running     bool
	logger      *slog.Logger
	interceptor *logging.RequestInterceptor
}

// NewStdioTransport creates a new stdio transport
func NewStdioTransport() *StdioTransport {
	logger := logging.GetGlobalLogger("transport.stdio")
	return &StdioTransport{
		scanner:     bufio.NewScanner(os.Stdin),
		logger:      logger,
		interceptor: logging.NewRequestInterceptor(logger),
	}
}

// Start begins listening for requests on stdin
func (t *StdioTransport) Start(ctx context.Context, handler RequestHandler) error {
	t.running = true
	
	// Log transport startup
	t.logger.InfoContext(ctx, "StdIO transport starting",
		slog.String("transport", "stdio"),
	)
	
	for t.running && t.scanner.Scan() {
		// Check context cancellation
		select {
		case <-ctx.Done():
			t.logger.InfoContext(ctx, "StdIO transport context cancelled")
			return ctx.Err()
		default:
		}
		
		line := strings.TrimSpace(t.scanner.Text())
		if line == "" {
			continue
		}
		
		// Create request context for this operation
		requestCtx := logging.NewRequestContext(ctx, "HandleStdIORequest")
		
		// Parse JSON-RPC request
		var req JSONRPCRequest
		if err := json.Unmarshal([]byte(line), &req); err != nil {
			t.logger.ErrorContext(requestCtx, "Failed to parse JSON-RPC request",
				slog.String("raw_input", line),
				slog.String("error", err.Error()),
			)
			
			// Send parse error response using error mapper
			parseErr := errors.Wrap(err, errors.ErrCodeTransportInvalidJSON, "Invalid JSON format")
			resp := ToJSONRPCResponse(nil, parseErr)
			t.sendResponse(requestCtx, resp)
			continue
		}
		
		// Log incoming request
		t.logger.InfoContext(requestCtx, "Processing JSON-RPC request",
			slog.String("method", req.Method),
			slog.Any("id", req.ID),
		)
		
		// Handle the request with timing
		startTime := time.Now()
		resp := handler(requestCtx, &req)
		duration := time.Since(startTime)
		
		// Log response
		if resp.Error != nil {
			t.logger.WarnContext(requestCtx, "Request completed with error",
				slog.String("method", req.Method),
				slog.Any("id", req.ID),
				slog.Duration("duration", duration),
				slog.String("error", resp.Error.Message),
			)
		} else {
			t.logger.InfoContext(requestCtx, "Request completed successfully",
				slog.String("method", req.Method),
				slog.Any("id", req.ID),
				slog.Duration("duration", duration),
			)
		}
		
		t.sendResponse(requestCtx, resp)
	}
	
	if err := t.scanner.Err(); err != nil && err != io.EOF {
		t.logger.ErrorContext(ctx, "Error reading from stdin",
			slog.String("error", err.Error()),
		)
		return fmt.Errorf("error reading from stdin: %w", err)
	}
	
	t.logger.InfoContext(ctx, "StdIO transport stopped")
	return nil
}

// Stop gracefully shuts down the transport
func (t *StdioTransport) Stop(ctx context.Context) error {
	t.logger.InfoContext(ctx, "StdIO transport stopping")
	t.running = false
	return nil
}

// Name returns the name of the transport
func (t *StdioTransport) Name() string {
	return "stdio"
}

// sendResponse sends a JSON-RPC response to stdout with proper error handling
func (t *StdioTransport) sendResponse(ctx context.Context, resp *JSONRPCResponse) {
	timer := logging.StartTimer(ctx, t.logger, "sendResponse")
	defer timer.End()
	
	t.logger.DebugContext(ctx, "Sending response",
		slog.Any("response_id", resp.ID),
		slog.Bool("has_error", resp.Error != nil),
	)
	
	respBytes, err := json.Marshal(resp)
	if err != nil {
		t.logger.ErrorContext(ctx, "Failed to marshal response",
			slog.Any("response_id", resp.ID),
			slog.String("error", err.Error()),
		)
		
		// Critical failure - send fallback response using error mapper
		marshalErr := errors.Wrap(err, errors.ErrCodeTransportMarshal, "Failed to serialize response")
		fallbackResp := ToJSONRPCResponse(resp.ID, marshalErr)
		if fallbackBytes, fallbackErr := json.Marshal(fallbackResp); fallbackErr == nil {
			fmt.Println(string(fallbackBytes))
		} else {
			t.logger.ErrorContext(ctx, "Failed to marshal fallback response",
				slog.String("error", fallbackErr.Error()),
			)
			
			// Ultimate fallback - hardcoded error
			ultimateResp := CreateFallbackErrorResponse(resp.ID, "Critical serialization error")
			if ultimateBytes, ultimateErr := json.Marshal(ultimateResp); ultimateErr == nil {
				fmt.Println(string(ultimateBytes))
			}
		}
		return
	}
	
	fmt.Println(string(respBytes))
	
	t.logger.DebugContext(ctx, "Response sent successfully",
		slog.Any("response_id", resp.ID),
		slog.Int("response_size", len(respBytes)),
	)
}