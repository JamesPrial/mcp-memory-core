package errors

import (
	"encoding/json"
	"fmt"
)

// ErrorCode represents a standardized error code
type ErrorCode string

// Standard error codes organized by category
const (
	// Storage errors (1xxx)
	ErrCodeStorageNotFound      ErrorCode = "STORAGE_NOT_FOUND"
	ErrCodeStorageConflict       ErrorCode = "STORAGE_CONFLICT"
	ErrCodeStorageTimeout        ErrorCode = "STORAGE_TIMEOUT"
	ErrCodeStorageConnection     ErrorCode = "STORAGE_CONNECTION"
	ErrCodeStorageTransaction    ErrorCode = "STORAGE_TRANSACTION"
	ErrCodeStorageConstraint     ErrorCode = "STORAGE_CONSTRAINT"
	ErrCodeStorageInvalidQuery   ErrorCode = "STORAGE_INVALID_QUERY"
	ErrCodeStorageInitialization ErrorCode = "STORAGE_INITIALIZATION"

	// Validation errors (2xxx)
	ErrCodeValidationRequired    ErrorCode = "VALIDATION_REQUIRED"
	ErrCodeValidationInvalid     ErrorCode = "VALIDATION_INVALID"
	ErrCodeValidationFormat      ErrorCode = "VALIDATION_FORMAT"
	ErrCodeValidationRange       ErrorCode = "VALIDATION_RANGE"
	ErrCodeValidationDuplicate   ErrorCode = "VALIDATION_DUPLICATE"
	ErrCodeValidationSize        ErrorCode = "VALIDATION_SIZE"
	ErrCodeValidationType        ErrorCode = "VALIDATION_TYPE"
	ErrCodeValidationConstraint  ErrorCode = "VALIDATION_CONSTRAINT"

	// Business logic errors (3xxx)
	ErrCodeEntityNotFound        ErrorCode = "ENTITY_NOT_FOUND"
	ErrCodeEntityAlreadyExists   ErrorCode = "ENTITY_ALREADY_EXISTS"
	ErrCodeRelationNotFound      ErrorCode = "RELATION_NOT_FOUND"
	ErrCodeRelationAlreadyExists ErrorCode = "RELATION_ALREADY_EXISTS"
	ErrCodeObservationNotFound   ErrorCode = "OBSERVATION_NOT_FOUND"
	ErrCodeInvalidOperation      ErrorCode = "INVALID_OPERATION"
	ErrCodeStateConflict         ErrorCode = "STATE_CONFLICT"

	// Authorization errors (4xxx)
	ErrCodeAuthUnauthorized      ErrorCode = "AUTH_UNAUTHORIZED"
	ErrCodeAuthForbidden         ErrorCode = "AUTH_FORBIDDEN"
	ErrCodeAuthTokenInvalid      ErrorCode = "AUTH_TOKEN_INVALID"
	ErrCodeAuthTokenExpired      ErrorCode = "AUTH_TOKEN_EXPIRED"
	ErrCodeAuthInsufficientScope ErrorCode = "AUTH_INSUFFICIENT_SCOPE"

	// Transport errors (5xxx)
	ErrCodeTransportMarshal      ErrorCode = "TRANSPORT_MARSHAL"
	ErrCodeTransportUnmarshal    ErrorCode = "TRANSPORT_UNMARSHAL"
	ErrCodeTransportInvalidJSON  ErrorCode = "TRANSPORT_INVALID_JSON"
	ErrCodeTransportMethodNotFound ErrorCode = "TRANSPORT_METHOD_NOT_FOUND"
	ErrCodeTransportInvalidParams ErrorCode = "TRANSPORT_INVALID_PARAMS"
	ErrCodeTransportTimeout      ErrorCode = "TRANSPORT_TIMEOUT"

	// System errors (9xxx)
	ErrCodeInternal              ErrorCode = "INTERNAL_ERROR"
	ErrCodeNotImplemented        ErrorCode = "NOT_IMPLEMENTED"
	ErrCodeServiceUnavailable    ErrorCode = "SERVICE_UNAVAILABLE"
	ErrCodeContextCanceled       ErrorCode = "CONTEXT_CANCELED"
	ErrCodeContextTimeout        ErrorCode = "CONTEXT_TIMEOUT"
	ErrCodePanic                 ErrorCode = "PANIC_RECOVERED"
	ErrCodeConfiguration         ErrorCode = "CONFIGURATION_ERROR"
	ErrCodeResourceExhausted     ErrorCode = "RESOURCE_EXHAUSTED"
)

// AppError represents a standardized application error
type AppError struct {
	Code     ErrorCode   `json:"code"`
	Message  string      `json:"message"`
	Details  interface{} `json:"details,omitempty"`
	Internal error       `json:"-"` // Internal error not exposed to clients
	Stack    []string    `json:"-"` // Stack trace for debugging
}

// Error implements the error interface
func (e *AppError) Error() string {
	return e.Message
}

// Unwrap returns the wrapped error
func (e *AppError) Unwrap() error {
	return e.Internal
}

// WithDetails adds details to the error
func (e *AppError) WithDetails(details interface{}) *AppError {
	e.Details = details
	return e
}

// WithStack adds a stack trace entry
func (e *AppError) WithStack(entry string) *AppError {
	e.Stack = append(e.Stack, entry)
	return e
}

// ToJSON returns a JSON representation safe for clients
func (e *AppError) ToJSON() ([]byte, error) {
	return json.Marshal(struct {
		Code    ErrorCode   `json:"code"`
		Message string      `json:"message"`
		Details interface{} `json:"details,omitempty"`
	}{
		Code:    e.Code,
		Message: e.Message,
		Details: e.Details,
	})
}

// New creates a new AppError
func New(code ErrorCode, message string) *AppError {
	return &AppError{
		Code:    code,
		Message: message,
		Stack:   make([]string, 0),
	}
}

// Newf creates a new AppError with formatted message
func Newf(code ErrorCode, format string, args ...interface{}) *AppError {
	return &AppError{
		Code:    code,
		Message: fmt.Sprintf(format, args...),
		Stack:   make([]string, 0),
	}
}

// Wrap wraps an existing error with an AppError
func Wrap(err error, code ErrorCode, message string) *AppError {
	if err == nil {
		return nil
	}
	
	// If already an AppError, preserve the chain
	if appErr, ok := err.(*AppError); ok {
		return &AppError{
			Code:     code,
			Message:  message,
			Internal: appErr,
			Stack:    appErr.Stack,
		}
	}
	
	return &AppError{
		Code:     code,
		Message:  message,
		Internal: err,
		Stack:    make([]string, 0),
	}
}

// Wrapf wraps an existing error with formatted message
func Wrapf(err error, code ErrorCode, format string, args ...interface{}) *AppError {
	if err == nil {
		return nil
	}
	
	return Wrap(err, code, fmt.Sprintf(format, args...))
}

// Is checks if an error has a specific error code
func Is(err error, code ErrorCode) bool {
	if err == nil {
		return false
	}
	
	appErr, ok := err.(*AppError)
	if !ok {
		return false
	}
	
	return appErr.Code == code
}

// IsAny checks if an error matches any of the provided codes
func IsAny(err error, codes ...ErrorCode) bool {
	for _, code := range codes {
		if Is(err, code) {
			return true
		}
	}
	return false
}

// GetCode extracts the error code from an error
func GetCode(err error) ErrorCode {
	if err == nil {
		return ""
	}
	
	appErr, ok := err.(*AppError)
	if !ok {
		return ErrCodeInternal
	}
	
	return appErr.Code
}

// GetMessage returns a safe message for the client
func GetMessage(err error) string {
	if err == nil {
		return ""
	}
	
	appErr, ok := err.(*AppError)
	if !ok {
		return "An internal error occurred"
	}
	
	return appErr.Message
}

// GetInternal returns the internal error for logging
func GetInternal(err error) error {
	if err == nil {
		return nil
	}
	
	appErr, ok := err.(*AppError)
	if !ok {
		return err
	}
	
	if appErr.Internal != nil {
		return appErr.Internal
	}
	
	return appErr
}

// NotFound creates a not found error
func NotFound(resource string) *AppError {
	return Newf(ErrCodeEntityNotFound, "%s not found", resource)
}

// AlreadyExists creates an already exists error
func AlreadyExists(resource string) *AppError {
	return Newf(ErrCodeEntityAlreadyExists, "%s already exists", resource)
}

// ValidationRequired creates a validation required error
func ValidationRequired(field string) *AppError {
	return Newf(ErrCodeValidationRequired, "%s is required", field)
}

// ValidationInvalid creates a validation invalid error
func ValidationInvalid(field, reason string) *AppError {
	return Newf(ErrCodeValidationInvalid, "%s is invalid: %s", field, reason)
}

// Internal creates an internal error with a safe message
func Internal(internalErr error) *AppError {
	return Wrap(internalErr, ErrCodeInternal, "An internal error occurred")
}

// Internalf creates an internal error with formatted safe message
func Internalf(internalErr error, format string, args ...interface{}) *AppError {
	return Wrap(internalErr, ErrCodeInternal, fmt.Sprintf(format, args...))
}