package transport

import (
	"net/http"
	"testing"

	"github.com/JamesPrial/mcp-memory-core/pkg/errors"
)

func TestToJSONRPCError(t *testing.T) {
	tests := []struct {
		name         string
		err          error
		expectedCode int
		expectedData string
	}{
		{
			name:         "nil error",
			err:          nil,
			expectedCode: 0,
			expectedData: "",
		},
		{
			name:         "transport method not found",
			err:          errors.New(errors.ErrCodeTransportMethodNotFound, "Method not found"),
			expectedCode: -32601,
			expectedData: "TRANSPORT_METHOD_NOT_FOUND",
		},
		{
			name:         "validation required",
			err:          errors.New(errors.ErrCodeValidationRequired, "Field is required"),
			expectedCode: -32602,
			expectedData: "VALIDATION_REQUIRED",
		},
		{
			name:         "entity not found",
			err:          errors.New(errors.ErrCodeEntityNotFound, "Entity not found"),
			expectedCode: -32001,
			expectedData: "ENTITY_NOT_FOUND",
		},
		{
			name:         "internal error",
			err:          errors.New(errors.ErrCodeInternal, "Internal error occurred"),
			expectedCode: -32603,
			expectedData: "INTERNAL_ERROR",
		},
		{
			name:         "auth unauthorized",
			err:          errors.New(errors.ErrCodeAuthUnauthorized, "Unauthorized"),
			expectedCode: -32004,
			expectedData: "AUTH_UNAUTHORIZED",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ToJSONRPCError(tt.err)

			if tt.err == nil {
				if result.Code != 0 || result.Message != "" {
					t.Errorf("Expected empty error for nil input, got code=%d, message=%s", result.Code, result.Message)
				}
				return
			}

			if result.Code != tt.expectedCode {
				t.Errorf("Expected code %d, got %d", tt.expectedCode, result.Code)
			}

			if result.Message != tt.err.Error() {
				t.Errorf("Expected message '%s', got '%s'", tt.err.Error(), result.Message)
			}

			if data, ok := result.Data.(map[string]interface{}); ok {
				if errorCode, exists := data["error_code"]; exists {
					if errorCode.(string) != tt.expectedData {
						t.Errorf("Expected error_code '%s', got '%s'", tt.expectedData, errorCode.(string))
					}
				} else {
					t.Error("Expected error_code in data")
				}
			} else {
				t.Error("Expected data to be a map")
			}
		})
	}
}

func TestToJSONRPCResponse(t *testing.T) {
	tests := []struct {
		name string
		id   interface{}
		err  error
	}{
		{
			name: "success response",
			id:   "test-id",
			err:  nil,
		},
		{
			name: "error response",
			id:   123,
			err:  errors.New(errors.ErrCodeValidationRequired, "Name is required"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ToJSONRPCResponse(tt.id, tt.err)

			if result.JSONRPC != "2.0" {
				t.Errorf("Expected JSONRPC '2.0', got '%s'", result.JSONRPC)
			}

			if result.ID != tt.id {
				t.Errorf("Expected ID %v, got %v", tt.id, result.ID)
			}

			if tt.err == nil {
				if result.Error != nil {
					t.Error("Expected no error in response")
				}
				if result.Result == nil {
					t.Error("Expected result in success response")
				}
			} else {
				if result.Error == nil {
					t.Error("Expected error in response")
				}
				if result.Result != nil {
					t.Error("Expected no result in error response")
				}
			}
		})
	}
}

func TestToHTTPStatusCode(t *testing.T) {
	tests := []struct {
		name         string
		err          error
		expectedCode int
	}{
		{
			name:         "nil error",
			err:          nil,
			expectedCode: http.StatusOK,
		},
		{
			name:         "transport invalid JSON - should return 400",
			err:          errors.New(errors.ErrCodeTransportInvalidJSON, "Invalid JSON"),
			expectedCode: http.StatusBadRequest,
		},
		{
			name:         "transport timeout - should return 408",
			err:          errors.New(errors.ErrCodeTransportTimeout, "Timeout"),
			expectedCode: http.StatusRequestTimeout,
		},
		{
			name:         "service unavailable - should return 503",
			err:          errors.New(errors.ErrCodeServiceUnavailable, "Service unavailable"),
			expectedCode: http.StatusServiceUnavailable,
		},
		{
			name:         "resource exhausted - should return 429",
			err:          errors.New(errors.ErrCodeResourceExhausted, "Too many requests"),
			expectedCode: http.StatusTooManyRequests,
		},
		{
			name:         "application error - should return 200",
			err:          errors.New(errors.ErrCodeEntityNotFound, "Entity not found"),
			expectedCode: http.StatusOK,
		},
		{
			name:         "validation error - should return 200",
			err:          errors.New(errors.ErrCodeValidationRequired, "Field required"),
			expectedCode: http.StatusOK,
		},
		{
			name:         "auth error - should return 200",
			err:          errors.New(errors.ErrCodeAuthUnauthorized, "Unauthorized"),
			expectedCode: http.StatusOK,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ToHTTPStatusCode(tt.err)

			if result != tt.expectedCode {
				t.Errorf("Expected status code %d, got %d", tt.expectedCode, result)
			}
		})
	}
}

func TestCreateFallbackErrorResponse(t *testing.T) {
	tests := []struct {
		name        string
		id          interface{}
		message     string
		expectedMsg string
	}{
		{
			name:        "with message",
			id:          "test-id",
			message:     "Custom error message",
			expectedMsg: "Custom error message",
		},
		{
			name:        "empty message",
			id:          123,
			message:     "",
			expectedMsg: "An unexpected error occurred",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := CreateFallbackErrorResponse(tt.id, tt.message)

			if result.JSONRPC != "2.0" {
				t.Errorf("Expected JSONRPC '2.0', got '%s'", result.JSONRPC)
			}

			if result.ID != tt.id {
				t.Errorf("Expected ID %v, got %v", tt.id, result.ID)
			}

			if result.Error == nil {
				t.Fatal("Expected error in fallback response")
			}

			if result.Error.Code != -32603 {
				t.Errorf("Expected error code -32603, got %d", result.Error.Code)
			}

			if result.Error.Message != tt.expectedMsg {
				t.Errorf("Expected message '%s', got '%s'", tt.expectedMsg, result.Error.Message)
			}

			if data, ok := result.Error.Data.(map[string]interface{}); ok {
				if errorCode, exists := data["error_code"]; !exists || errorCode != "FALLBACK_ERROR" {
					t.Error("Expected error_code 'FALLBACK_ERROR' in data")
				}
			} else {
				t.Error("Expected data to be a map")
			}
		})
	}
}

func TestSafeErrorMessage(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected string
	}{
		{
			name:     "nil error",
			err:      nil,
			expected: "",
		},
		{
			name:     "app error with message",
			err:      errors.New(errors.ErrCodeEntityNotFound, "Entity not found"),
			expected: "Entity not found",
		},
		{
			name:     "wrapped error",
			err:      errors.Wrap(errors.New(errors.ErrCodeInternal, ""), errors.ErrCodeEntityNotFound, "User not found"),
			expected: "User not found",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := SafeErrorMessage(tt.err)

			if result != tt.expected {
				t.Errorf("Expected message '%s', got '%s'", tt.expected, result)
			}
		})
	}
}

func TestLoggableError(t *testing.T) {
	tests := []struct {
		name        string
		err         error
		shouldBeNil bool
	}{
		{
			name:        "nil error",
			err:         nil,
			shouldBeNil: true,
		},
		{
			name:        "app error",
			err:         errors.New(errors.ErrCodeEntityNotFound, "Entity not found"),
			shouldBeNil: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := LoggableError(tt.err)

			if tt.shouldBeNil && result != nil {
				t.Error("Expected nil result for nil error")
			}

			if !tt.shouldBeNil && result == nil {
				t.Error("Expected non-nil result for non-nil error")
			}

			if result != nil {
				// Just verify it returns a string - the exact format can vary
				if result.Error() == "" {
					t.Error("Expected non-empty error string")
				}
			}
		})
	}
}