package errors

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"testing"
)

func TestNew(t *testing.T) {
	tests := []struct {
		name    string
		code    ErrorCode
		message string
	}{
		{
			name:    "creates error with code and message",
			code:    ErrCodeEntityNotFound,
			message: "user not found",
		},
		{
			name:    "creates validation error",
			code:    ErrCodeValidationRequired,
			message: "email is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := New(tt.code, tt.message)
			
			if err.Code != tt.code {
				t.Errorf("expected code %s, got %s", tt.code, err.Code)
			}
			
			if err.Message != tt.message {
				t.Errorf("expected message %s, got %s", tt.message, err.Message)
			}
			
			if err.Internal != nil {
				t.Error("expected Internal to be nil")
			}
			
			if err.Error() != tt.message {
				t.Errorf("Error() should return message")
			}
		})
	}
}

func TestNewf(t *testing.T) {
	err := Newf(ErrCodeValidationInvalid, "field %s has invalid value: %d", "age", -1)
	expected := "field age has invalid value: -1"
	
	if err.Message != expected {
		t.Errorf("expected message %s, got %s", expected, err.Message)
	}
}

func TestWrap(t *testing.T) {
	originalErr := errors.New("database connection failed")
	
	tests := []struct {
		name     string
		err      error
		code     ErrorCode
		message  string
		checkNil bool
	}{
		{
			name:    "wraps standard error",
			err:     originalErr,
			code:    ErrCodeStorageConnection,
			message: "Failed to connect to database",
		},
		{
			name:     "returns nil for nil error",
			err:      nil,
			code:     ErrCodeInternal,
			message:  "should not appear",
			checkNil: true,
		},
		{
			name:    "preserves AppError chain",
			err:     New(ErrCodeEntityNotFound, "original"),
			code:    ErrCodeInternal,
			message: "wrapped",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			wrapped := Wrap(tt.err, tt.code, tt.message)
			
			if tt.checkNil {
				if wrapped != nil {
					t.Error("expected nil for nil error")
				}
				return
			}
			
			if wrapped.Code != tt.code {
				t.Errorf("expected code %s, got %s", tt.code, wrapped.Code)
			}
			
			if wrapped.Message != tt.message {
				t.Errorf("expected message %s, got %s", tt.message, wrapped.Message)
			}
			
			if tt.err != nil && wrapped.Internal == nil {
				t.Error("expected Internal to be set")
			}
			
			// Test Unwrap
			if unwrapped := wrapped.Unwrap(); tt.err != nil && unwrapped == nil {
				t.Error("Unwrap should return the internal error")
			}
		})
	}
}

func TestIs(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		code     ErrorCode
		expected bool
	}{
		{
			name:     "matches AppError code",
			err:      New(ErrCodeEntityNotFound, "not found"),
			code:     ErrCodeEntityNotFound,
			expected: true,
		},
		{
			name:     "doesn't match different code",
			err:      New(ErrCodeEntityNotFound, "not found"),
			code:     ErrCodeInternal,
			expected: false,
		},
		{
			name:     "returns false for nil",
			err:      nil,
			code:     ErrCodeInternal,
			expected: false,
		},
		{
			name:     "returns false for non-AppError",
			err:      errors.New("standard error"),
			code:     ErrCodeInternal,
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := Is(tt.err, tt.code)
			if result != tt.expected {
				t.Errorf("expected %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestIsAny(t *testing.T) {
	err := New(ErrCodeEntityNotFound, "not found")
	
	if !IsAny(err, ErrCodeEntityNotFound, ErrCodeRelationNotFound) {
		t.Error("should match one of the codes")
	}
	
	if IsAny(err, ErrCodeInternal, ErrCodeValidationRequired) {
		t.Error("should not match any of the codes")
	}
}

func TestGetCode(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected ErrorCode
	}{
		{
			name:     "returns code from AppError",
			err:      New(ErrCodeEntityNotFound, "not found"),
			expected: ErrCodeEntityNotFound,
		},
		{
			name:     "returns internal for standard error",
			err:      errors.New("standard"),
			expected: ErrCodeInternal,
		},
		{
			name:     "returns empty for nil",
			err:      nil,
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			code := GetCode(tt.err)
			if code != tt.expected {
				t.Errorf("expected %s, got %s", tt.expected, code)
			}
		})
	}
}

func TestGetMessage(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected string
	}{
		{
			name:     "returns message from AppError",
			err:      New(ErrCodeEntityNotFound, "user not found"),
			expected: "user not found",
		},
		{
			name:     "returns safe message for standard error",
			err:      errors.New("internal details"),
			expected: "An internal error occurred",
		},
		{
			name:     "returns empty for nil",
			err:      nil,
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			msg := GetMessage(tt.err)
			if msg != tt.expected {
				t.Errorf("expected %s, got %s", tt.expected, msg)
			}
		})
	}
}

func TestGetInternal(t *testing.T) {
	originalErr := errors.New("original")
	wrapped := Wrap(originalErr, ErrCodeInternal, "wrapped")
	
	internal := GetInternal(wrapped)
	if internal != originalErr {
		t.Error("should return the original error")
	}
	
	appErr := New(ErrCodeEntityNotFound, "not found")
	internal = GetInternal(appErr)
	if internal != appErr {
		t.Error("should return self when no internal error")
	}
	
	if GetInternal(nil) != nil {
		t.Error("should return nil for nil error")
	}
	
	stdErr := errors.New("standard")
	if GetInternal(stdErr) != stdErr {
		t.Error("should return original for non-AppError")
	}
}

func TestHelperFunctions(t *testing.T) {
	t.Run("NotFound", func(t *testing.T) {
		err := NotFound("user")
		if err.Code != ErrCodeEntityNotFound {
			t.Error("should have entity not found code")
		}
		if !strings.Contains(err.Message, "user not found") {
			t.Error("should contain resource name")
		}
	})
	
	t.Run("AlreadyExists", func(t *testing.T) {
		err := AlreadyExists("entity")
		if err.Code != ErrCodeEntityAlreadyExists {
			t.Error("should have already exists code")
		}
		if !strings.Contains(err.Message, "entity already exists") {
			t.Error("should contain resource name")
		}
	})
	
	t.Run("ValidationRequired", func(t *testing.T) {
		err := ValidationRequired("email")
		if err.Code != ErrCodeValidationRequired {
			t.Error("should have validation required code")
		}
		if !strings.Contains(err.Message, "email is required") {
			t.Error("should contain field name")
		}
	})
	
	t.Run("ValidationInvalid", func(t *testing.T) {
		err := ValidationInvalid("age", "must be positive")
		if err.Code != ErrCodeValidationInvalid {
			t.Error("should have validation invalid code")
		}
		if !strings.Contains(err.Message, "age") || !strings.Contains(err.Message, "must be positive") {
			t.Error("should contain field and reason")
		}
	})
	
	t.Run("Internal", func(t *testing.T) {
		originalErr := errors.New("database crashed")
		err := Internal(originalErr)
		if err.Code != ErrCodeInternal {
			t.Error("should have internal code")
		}
		if err.Message != "An internal error occurred" {
			t.Error("should have safe message")
		}
		if err.Internal != originalErr {
			t.Error("should preserve original error")
		}
	})
}

func TestWithDetails(t *testing.T) {
	err := New(ErrCodeValidationInvalid, "invalid input")
	details := map[string]string{"field": "email", "reason": "invalid format"}
	
	err.WithDetails(details)
	
	if err.Details == nil {
		t.Error("details should be set")
	}
	
	if d, ok := err.Details.(map[string]string); !ok || d["field"] != "email" {
		t.Error("details not preserved correctly")
	}
}

func TestWithStack(t *testing.T) {
	err := New(ErrCodeInternal, "error")
	err.WithStack("function1").WithStack("function2")
	
	if len(err.Stack) != 2 {
		t.Errorf("expected 2 stack entries, got %d", len(err.Stack))
	}
	
	if err.Stack[0] != "function1" || err.Stack[1] != "function2" {
		t.Error("stack entries not correct")
	}
}

func TestToJSON(t *testing.T) {
	err := New(ErrCodeValidationInvalid, "invalid email").
		WithDetails(map[string]string{"field": "email"})
	
	jsonBytes, jsonErr := err.ToJSON()
	if jsonErr != nil {
		t.Fatalf("failed to marshal to JSON: %v", jsonErr)
	}
	
	var result map[string]interface{}
	if unmarshalErr := json.Unmarshal(jsonBytes, &result); unmarshalErr != nil {
		t.Fatalf("failed to unmarshal JSON: %v", unmarshalErr)
	}
	
	if result["code"] != string(ErrCodeValidationInvalid) {
		t.Error("code not in JSON")
	}
	
	if result["message"] != "invalid email" {
		t.Error("message not in JSON")
	}
	
	if result["details"] == nil {
		t.Error("details not in JSON")
	}
	
	// Should not include internal error or stack
	if _, hasInternal := result["internal"]; hasInternal {
		t.Error("internal error should not be in JSON")
	}
	
	if _, hasStack := result["stack"]; hasStack {
		t.Error("stack should not be in JSON")
	}
}

func TestHTTPStatusFromCode(t *testing.T) {
	tests := []struct {
		code     ErrorCode
		expected int
	}{
		// 400 Bad Request
		{ErrCodeValidationRequired, http.StatusBadRequest},
		{ErrCodeValidationInvalid, http.StatusBadRequest},
		{ErrCodeTransportInvalidJSON, http.StatusBadRequest},
		
		// 401 Unauthorized
		{ErrCodeAuthUnauthorized, http.StatusUnauthorized},
		{ErrCodeAuthTokenInvalid, http.StatusUnauthorized},
		
		// 403 Forbidden
		{ErrCodeAuthForbidden, http.StatusForbidden},
		
		// 404 Not Found
		{ErrCodeEntityNotFound, http.StatusNotFound},
		{ErrCodeStorageNotFound, http.StatusNotFound},
		
		// 409 Conflict
		{ErrCodeEntityAlreadyExists, http.StatusConflict},
		{ErrCodeStorageConflict, http.StatusConflict},
		
		// 408 Timeout
		{ErrCodeStorageTimeout, http.StatusRequestTimeout},
		{ErrCodeContextTimeout, http.StatusRequestTimeout},
		
		// 500 Internal Server Error
		{ErrCodeInternal, http.StatusInternalServerError},
		{ErrCodePanic, http.StatusInternalServerError},
		{ErrCodeStorageConnection, http.StatusInternalServerError},
		
		// 501 Not Implemented
		{ErrCodeNotImplemented, http.StatusNotImplemented},
		
		// 503 Service Unavailable
		{ErrCodeServiceUnavailable, http.StatusServiceUnavailable},
	}

	for _, tt := range tests {
		t.Run(string(tt.code), func(t *testing.T) {
			status := HTTPStatusFromCode(tt.code)
			if status != tt.expected {
				t.Errorf("expected status %d, got %d", tt.expected, status)
			}
		})
	}
}

func TestHTTPStatusCode(t *testing.T) {
	err := New(ErrCodeEntityNotFound, "not found")
	if status := HTTPStatusCode(err); status != http.StatusNotFound {
		t.Errorf("expected 404, got %d", status)
	}
	
	if status := HTTPStatusCode(nil); status != http.StatusOK {
		t.Errorf("expected 200 for nil, got %d", status)
	}
}

func TestToHTTPError(t *testing.T) {
	appErr := New(ErrCodeValidationRequired, "email required").
		WithDetails(map[string]string{"field": "email"})
	
	httpErr := ToHTTPError(appErr)
	
	if httpErr.Status != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", httpErr.Status)
	}
	
	if httpErr.Code != ErrCodeValidationRequired {
		t.Error("code not preserved")
	}
	
	if httpErr.Message != "email required" {
		t.Error("message not preserved")
	}
	
	if httpErr.Details == nil {
		t.Error("details not preserved")
	}
	
	// Test with standard error
	stdErr := errors.New("standard error")
	httpErr = ToHTTPError(stdErr)
	if httpErr.Status != http.StatusInternalServerError {
		t.Error("standard error should map to 500")
	}
	if httpErr.Message != "An internal error occurred" {
		t.Error("standard error should have safe message")
	}
}

func TestIsClientError(t *testing.T) {
	if !IsClientError(New(ErrCodeValidationRequired, "required")) {
		t.Error("validation error should be client error")
	}
	
	if IsClientError(New(ErrCodeInternal, "internal")) {
		t.Error("internal error should not be client error")
	}
}

func TestIsServerError(t *testing.T) {
	if !IsServerError(New(ErrCodeInternal, "internal")) {
		t.Error("internal error should be server error")
	}
	
	if IsServerError(New(ErrCodeValidationRequired, "required")) {
		t.Error("validation error should not be server error")
	}
}

func TestLogger(t *testing.T) {
	// Create a test logger that captures output
	var captured []string
	var testWriter strings.Builder
	testLogger := slog.New(slog.NewTextHandler(&testWriter, &slog.HandlerOptions{
		ReplaceAttr: func(groups []string, a slog.Attr) slog.Attr {
			if a.Key == "msg" || a.Key == "error_code" || a.Key == "operation" {
				captured = append(captured, fmt.Sprintf("%s=%v", a.Key, a.Value))
			}
			return a
		},
	}))
	
	logger := NewLogger(testLogger)
	ctx := context.Background()
	
	t.Run("LogError with AppError", func(t *testing.T) {
		captured = nil
		appErr := New(ErrCodeEntityNotFound, "user not found")
		result := logger.LogError(ctx, appErr, "GetUser")
		
		if result != appErr {
			t.Error("should return the same error")
		}
		
		// Check that operation was logged
		found := false
		for _, c := range captured {
			if strings.Contains(c, "operation=GetUser") {
				found = true
				break
			}
		}
		if !found {
			t.Error("operation not logged")
		}
	})
	
	t.Run("LogError with standard error", func(t *testing.T) {
		captured = nil
		stdErr := errors.New("database error")
		result := logger.LogError(ctx, stdErr, "Query")
		
		appErr, ok := result.(*AppError)
		if !ok {
			t.Fatal("should return AppError")
		}
		
		if appErr.Code != ErrCodeInternal {
			t.Error("should wrap as internal error")
		}
		
		if appErr.Message != "An internal error occurred" {
			t.Error("should have safe message")
		}
	})
	
	t.Run("LogAndWrap", func(t *testing.T) {
		captured = nil
		stdErr := errors.New("connection failed")
		result := logger.LogAndWrap(ctx, stdErr, ErrCodeStorageConnection, "Database unavailable", "Connect")
		
		if result.Code != ErrCodeStorageConnection {
			t.Error("wrong error code")
		}
		
		if result.Message != "Database unavailable" {
			t.Error("wrong message")
		}
		
		if len(result.Stack) == 0 || result.Stack[0] != "Connect" {
			t.Error("operation not in stack")
		}
	})
	
	t.Run("LogPanic", func(t *testing.T) {
		captured = nil
		panicValue := "something went wrong"
		result := logger.LogPanic(ctx, panicValue, "HandleRequest")
		
		appErr, ok := result.(*AppError)
		if !ok {
			t.Fatal("should return AppError")
		}
		
		if appErr.Code != ErrCodePanic {
			t.Error("should have panic code")
		}
		
		if appErr.Message != "An unexpected error occurred" {
			t.Error("should have safe panic message")
		}
	})
	
	t.Run("extractRequestID", func(t *testing.T) {
		// Test with request_id in context
		ctxWithID := context.WithValue(ctx, "request_id", "req-123")
		logger.LogError(ctxWithID, New(ErrCodeInternal, "error"), "Operation")
		
		// This is mainly to ensure it doesn't panic
		// In a real scenario, we'd check the logged output
	})
}

func TestLoggerHelpers(t *testing.T) {
	// Test default logger functions
	ctx := context.Background()
	
	err := LogError(ctx, New(ErrCodeEntityNotFound, "not found"), "Find")
	if err == nil {
		t.Error("should return error")
	}
	
	wrapped := LogAndWrap(ctx, errors.New("db error"), ErrCodeStorageConnection, "connection failed", "Connect")
	if wrapped.Code != ErrCodeStorageConnection {
		t.Error("wrong code")
	}
	
	formatted := LogAndWrapf(ctx, errors.New("parse error"), ErrCodeValidationFormat, "Parse", "invalid format: %s", "email")
	if !strings.Contains(formatted.Message, "invalid format: email") {
		t.Error("format not applied")
	}
	
	panicErr := LogPanic(ctx, "panic value", "Handler")
	if panicErr == nil || GetCode(panicErr) != ErrCodePanic {
		t.Error("panic not handled correctly")
	}
}