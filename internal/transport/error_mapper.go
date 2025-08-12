package transport

import (
	"fmt"
	"net/http"

	"github.com/JamesPrial/mcp-memory-core/pkg/errors"
	"github.com/JamesPrial/mcp-memory-core/pkg/mcp"
)

// ToJSONRPCError maps internal AppError codes to JSON-RPC error codes and mcp.Error
func ToJSONRPCError(err error) mcp.Error {
	if err == nil {
		return mcp.Error{}
	}
	
	code := errors.GetCode(err)
	message := errors.GetMessage(err)
	
	// Map to JSON-RPC codes according to spec
	var jsonRPCCode int
	switch code {
	// Transport layer errors
	case errors.ErrCodeTransportMethodNotFound:
		jsonRPCCode = -32601 // Method not found
	case errors.ErrCodeTransportInvalidParams:
		jsonRPCCode = -32602 // Invalid params
	case errors.ErrCodeTransportInvalidJSON, errors.ErrCodeTransportMarshal, errors.ErrCodeTransportUnmarshal:
		jsonRPCCode = -32700 // Parse error
	case errors.ErrCodeTransportTimeout:
		jsonRPCCode = -32603 // Internal error
		
	// Validation errors - map to Invalid params
	case errors.ErrCodeValidationRequired, errors.ErrCodeValidationInvalid, 
		 errors.ErrCodeValidationFormat, errors.ErrCodeValidationRange, 
		 errors.ErrCodeValidationDuplicate, errors.ErrCodeValidationSize,
		 errors.ErrCodeValidationType, errors.ErrCodeValidationConstraint:
		jsonRPCCode = -32602 // Invalid params
		
	// System errors - map to Internal error
	case errors.ErrCodeInternal, errors.ErrCodePanic, errors.ErrCodeContextCanceled,
		 errors.ErrCodeContextTimeout, errors.ErrCodeServiceUnavailable,
		 errors.ErrCodeResourceExhausted, errors.ErrCodeConfiguration:
		jsonRPCCode = -32603 // Internal error
		
	// Custom application errors - use custom range -32000 to -32099
	case errors.ErrCodeEntityNotFound, errors.ErrCodeRelationNotFound, errors.ErrCodeObservationNotFound:
		jsonRPCCode = -32001 // Application: Not Found
	case errors.ErrCodeEntityAlreadyExists, errors.ErrCodeRelationAlreadyExists:
		jsonRPCCode = -32002 // Application: Already Exists
	case errors.ErrCodeInvalidOperation, errors.ErrCodeStateConflict:
		jsonRPCCode = -32003 // Application: Conflict
	case errors.ErrCodeAuthUnauthorized, errors.ErrCodeAuthTokenInvalid, errors.ErrCodeAuthTokenExpired:
		jsonRPCCode = -32004 // Application: Unauthorized
	case errors.ErrCodeAuthForbidden, errors.ErrCodeAuthInsufficientScope:
		jsonRPCCode = -32005 // Application: Forbidden
	case errors.ErrCodeStorageNotFound:
		jsonRPCCode = -32001 // Application: Not Found
	case errors.ErrCodeStorageConflict, errors.ErrCodeStorageConstraint:
		jsonRPCCode = -32003 // Application: Conflict
	case errors.ErrCodeStorageTimeout, errors.ErrCodeStorageConnection, 
		 errors.ErrCodeStorageTransaction, errors.ErrCodeStorageInvalidQuery,
		 errors.ErrCodeStorageInitialization:
		jsonRPCCode = -32603 // Internal error
	case errors.ErrCodeNotImplemented:
		jsonRPCCode = -32601 // Method not found
	default:
		jsonRPCCode = -32000 // Generic custom error
	}
	
	return mcp.Error{
		Code:    jsonRPCCode,
		Message: message,
		Data:    map[string]interface{}{"error_code": string(code)},
	}
}

// ToJSONRPCResponse creates a complete JSONRPCResponse with error
func ToJSONRPCResponse(id interface{}, err error) *JSONRPCResponse {
	if err == nil {
		return &JSONRPCResponse{
			JSONRPC: "2.0",
			ID:      id,
			Result:  map[string]interface{}{"success": true},
		}
	}
	
	mcpError := ToJSONRPCError(err)
	jsonRPCError := &JSONRPCError{
		Code:    mcpError.Code,
		Message: mcpError.Message,
		Data:    mcpError.Data,
	}
	
	return &JSONRPCResponse{
		JSONRPC: "2.0",
		ID:      id,
		Error:   jsonRPCError,
	}
}

// ToHTTPStatusCode maps error codes to appropriate HTTP status codes
// Following JSON-RPC over HTTP best practices:
// - Transport-level errors (parsing, malformed requests) return HTTP error codes
// - Application-level errors return HTTP 200 with JSON-RPC error in body
func ToHTTPStatusCode(err error) int {
	if err == nil {
		return http.StatusOK
	}
	
	code := errors.GetCode(err)
	
	switch code {
	// Transport-level errors - return HTTP error codes
	case errors.ErrCodeTransportInvalidJSON, errors.ErrCodeTransportMarshal, errors.ErrCodeTransportUnmarshal:
		return http.StatusBadRequest // Parse/serialization errors
	case errors.ErrCodeTransportTimeout:
		return http.StatusRequestTimeout
		
	// System-level errors that should return HTTP error codes
	case errors.ErrCodeServiceUnavailable:
		return http.StatusServiceUnavailable
	case errors.ErrCodeResourceExhausted:
		return http.StatusTooManyRequests
		
	// Critical system errors
	case errors.ErrCodePanic, errors.ErrCodeConfiguration:
		return http.StatusInternalServerError
		
	// All other errors are application-level and should return HTTP 200
	// with the error details in the JSON-RPC response body
	default:
		return http.StatusOK
	}
}

// CreateFallbackErrorResponse creates a safe fallback error response for critical failures
func CreateFallbackErrorResponse(id interface{}, message string) *JSONRPCResponse {
	if message == "" {
		message = "An unexpected error occurred"
	}
	
	return &JSONRPCResponse{
		JSONRPC: "2.0",
		ID:      id,
		Error: &JSONRPCError{
			Code:    -32603, // Internal error
			Message: message,
			Data:    map[string]interface{}{"error_code": "FALLBACK_ERROR"},
		},
	}
}

// SafeErrorMessage returns a client-safe error message, sanitizing internal details
func SafeErrorMessage(err error) string {
	if err == nil {
		return ""
	}
	
	// Use the errors package to get the safe message
	message := errors.GetMessage(err)
	
	// Additional sanitization - remove any internal details that might leak
	if message == "" {
		return "An internal error occurred"
	}
	
	return message
}

// LoggableError returns the full error details for logging (including internal error)
func LoggableError(err error) error {
	if err == nil {
		return nil
	}
	
	// Return the internal error for logging
	internal := errors.GetInternal(err)
	if internal != nil {
		return fmt.Errorf("error_code=%s message=%s internal=%v", 
			errors.GetCode(err), errors.GetMessage(err), internal)
	}
	
	return fmt.Errorf("error_code=%s message=%s", 
		errors.GetCode(err), errors.GetMessage(err))
}