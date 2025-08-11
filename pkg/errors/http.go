package errors

import (
	"net/http"
	"strings"
)

// HTTPStatusCode returns the appropriate HTTP status code for an error
func HTTPStatusCode(err error) int {
	if err == nil {
		return http.StatusOK
	}

	code := GetCode(err)
	return HTTPStatusFromCode(code)
}

// HTTPStatusFromCode returns the HTTP status for an error code
func HTTPStatusFromCode(code ErrorCode) int {
	switch code {
	// 400 Bad Request - Client sent invalid data
	case ErrCodeValidationRequired,
		ErrCodeValidationInvalid,
		ErrCodeValidationFormat,
		ErrCodeValidationRange,
		ErrCodeValidationSize,
		ErrCodeValidationType,
		ErrCodeValidationConstraint,
		ErrCodeTransportInvalidJSON,
		ErrCodeTransportInvalidParams,
		ErrCodeStorageInvalidQuery:
		return http.StatusBadRequest

	// 401 Unauthorized - Authentication required
	case ErrCodeAuthUnauthorized,
		ErrCodeAuthTokenInvalid,
		ErrCodeAuthTokenExpired:
		return http.StatusUnauthorized

	// 403 Forbidden - Authenticated but not authorized
	case ErrCodeAuthForbidden,
		ErrCodeAuthInsufficientScope:
		return http.StatusForbidden

	// 404 Not Found - Resource doesn't exist
	case ErrCodeEntityNotFound,
		ErrCodeRelationNotFound,
		ErrCodeObservationNotFound,
		ErrCodeStorageNotFound,
		ErrCodeTransportMethodNotFound:
		return http.StatusNotFound

	// 409 Conflict - Resource state conflict
	case ErrCodeEntityAlreadyExists,
		ErrCodeRelationAlreadyExists,
		ErrCodeStorageConflict,
		ErrCodeStateConflict,
		ErrCodeValidationDuplicate,
		ErrCodeStorageConstraint:
		return http.StatusConflict

	// 408 Request Timeout
	case ErrCodeStorageTimeout,
		ErrCodeTransportTimeout,
		ErrCodeContextTimeout:
		return http.StatusRequestTimeout

	// 422 Unprocessable Entity - Semantic errors
	case ErrCodeInvalidOperation,
		ErrCodeTransportMarshal,
		ErrCodeTransportUnmarshal:
		return http.StatusUnprocessableEntity

	// 429 Too Many Requests
	case ErrCodeResourceExhausted:
		return http.StatusTooManyRequests

	// 500 Internal Server Error
	case ErrCodeInternal,
		ErrCodePanic,
		ErrCodeStorageConnection,
		ErrCodeStorageTransaction,
		ErrCodeStorageInitialization,
		ErrCodeConfiguration:
		return http.StatusInternalServerError

	// 501 Not Implemented
	case ErrCodeNotImplemented:
		return http.StatusNotImplemented

	// 503 Service Unavailable
	case ErrCodeServiceUnavailable:
		return http.StatusServiceUnavailable

	// 499 Client Closed Request (non-standard but commonly used)
	case ErrCodeContextCanceled:
		return 499

	default:
		// Try to infer from prefix
		codeStr := string(code)
		switch {
		case strings.HasPrefix(codeStr, "VALIDATION_"):
			return http.StatusBadRequest
		case strings.HasPrefix(codeStr, "AUTH_"):
			return http.StatusUnauthorized
		case strings.HasPrefix(codeStr, "STORAGE_"):
			return http.StatusInternalServerError
		case strings.HasPrefix(codeStr, "TRANSPORT_"):
			return http.StatusBadRequest
		default:
			return http.StatusInternalServerError
		}
	}
}

// HTTPError represents an HTTP-specific error response
type HTTPError struct {
	Status  int         `json:"status"`
	Code    ErrorCode   `json:"code"`
	Message string      `json:"message"`
	Details interface{} `json:"details,omitempty"`
}

// ToHTTPError converts an error to an HTTP error response
func ToHTTPError(err error) HTTPError {
	if err == nil {
		return HTTPError{
			Status:  http.StatusOK,
			Message: "OK",
		}
	}

	appErr, ok := err.(*AppError)
	if !ok {
		// Wrap as internal error
		appErr = Internal(err)
	}

	return HTTPError{
		Status:  HTTPStatusFromCode(appErr.Code),
		Code:    appErr.Code,
		Message: appErr.Message,
		Details: appErr.Details,
	}
}

// IsClientError returns true if the error is a client error (4xx)
func IsClientError(err error) bool {
	status := HTTPStatusCode(err)
	return status >= 400 && status < 500
}

// IsServerError returns true if the error is a server error (5xx)
func IsServerError(err error) bool {
	status := HTTPStatusCode(err)
	return status >= 500 && status < 600
}