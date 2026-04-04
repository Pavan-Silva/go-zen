package zen

import (
	"encoding/json"
	"log"
	"net/http"
)

// HTTPError represents a structured HTTP error response in JSON format.
// It provides a standard way to return error information to clients with
// status codes, messages, optional details, and request IDs for tracing.
//
// For security, Details are automatically hidden for 5xx errors when sent via SendError.
// Use NewHTTPError or CommonErrors to create instances.
//
// Example:
//
//	err := zen.NewHTTPError(http.StatusBadRequest, "invalid input")
//	err = err.WithDetails(map[string]string{"field": "email"})
//	c.SendError(err)
type HTTPError struct {
	// Status HTTP status code (e.g., 400, 401, 404, 500)
	Status int `json:"status"`
	// Message human-readable error message
	Message string `json:"message"`
	// Details optional details for debugging (omitted for 5xx errors in production)
	Details any `json:"details,omitempty"`
	// RequestID optional request ID for tracking
	RequestID string `json:"request_id,omitempty"`
}

// Error implements the error interface, returning the message as a string.
func (e *HTTPError) Error() string {
	return e.Message
}

// NewHTTPError creates a new HTTPError with the given status and message.
// Use this to create custom errors or use CommonErrors for common cases.
//
// Example:
//
//	err := zen.NewHTTPError(http.StatusGone, "resource deleted")
func NewHTTPError(status int, message string) *HTTPError {
	return &HTTPError{
		Status:  status,
		Message: message,
	}
}

// WithDetails adds optional structured details to the error.
// Useful for providing field-level error information or debugging context.
// Details are hidden for 5xx errors when sent via SendError (security).
//
// Example:
//
//	err := zen.NewHTTPError(http.StatusBadRequest, "validation failed")
//	err = err.WithDetails(map[string][]string{
//	    "email": {"invalid format"},
//	    "age": {"must be >= 18"},
//	})
func (e *HTTPError) WithDetails(details any) *HTTPError {
	e.Details = details
	return e
}

// WithRequestID adds a request/trace ID for this error.
// Allows correlating client-side errors with server logs.
//
// Example:
//
//	requestID := c.Get("request_id").(string)
//	err := zen.CommonErrors.InternalError("failed to save")
//	err = err.WithRequestID(requestID)
func (e *HTTPError) WithRequestID(id string) *HTTPError {
	e.RequestID = id
	return e
}

// SendError encodes an HTTPError as JSON and writes it with the appropriate status.
// For 5xx errors, Details are automatically cleared for security (don't leak internals).
// Write errors are logged but not returned (indicating connection issues).
//
// Example:
//
//	if err := someOp(); err != nil {
//	    c.SendError(zen.CommonErrors.InternalError(err.Error()))
//	    return
//	}
func (c *Context) SendError(err *HTTPError) {
	// Don't expose internal error details for server errors
	if err.Status >= http.StatusInternalServerError {
		err.Details = nil
	}

	c.Response.Header().Set("Content-Type", "application/json")
	c.Response.WriteHeader(err.Status)

	if encErr := json.NewEncoder(c.Response).Encode(err); encErr != nil {
		log.Printf("zen: error response encode failed: %v", encErr)
	}
}

// CommonErrors provides pre-built error responses for common HTTP scenarios.
// Use these helpers for consistent error formatting across your application.
//
// Example:
//
//	if user == nil {
//	    c.SendError(zen.CommonErrors.NotFound("user"))
//	    return
//	}
//
//	if !authorized {
//	    c.SendError(zen.CommonErrors.Unauthorized())
//	    return
//	}
var CommonErrors = struct {
	// BadRequest creates a 400 error with a custom message
	BadRequest func(message string) *HTTPError
	// Unauthorized creates a 401 error
	Unauthorized func() *HTTPError
	// Forbidden creates a 403 error
	Forbidden func() *HTTPError
	// NotFound creates a 404 error with "{resource} not found"
	NotFound func(resource string) *HTTPError
	// Conflict creates a 409 error with a custom message
	Conflict func(message string) *HTTPError
	// InternalError creates a 500 error with a custom message
	InternalError func(message string) *HTTPError
	// ServiceUnavailable creates a 503 error
	ServiceUnavailable func() *HTTPError
}{
	BadRequest: func(message string) *HTTPError {
		return NewHTTPError(http.StatusBadRequest, message)
	},
	Unauthorized: func() *HTTPError {
		return NewHTTPError(http.StatusUnauthorized, "unauthorized")
	},
	Forbidden: func() *HTTPError {
		return NewHTTPError(http.StatusForbidden, "forbidden")
	},
	NotFound: func(resource string) *HTTPError {
		return NewHTTPError(http.StatusNotFound, resource+" not found")
	},
	Conflict: func(message string) *HTTPError {
		return NewHTTPError(http.StatusConflict, message)
	},
	InternalError: func(message string) *HTTPError {
		return NewHTTPError(http.StatusInternalServerError, message)
	},
	ServiceUnavailable: func() *HTTPError {
		return NewHTTPError(http.StatusServiceUnavailable, "service unavailable")
	},
}
