package zen

import (
	"encoding/json"
	"log"
	"net/http"
)

// HTTPError represents a structured HTTP error response.
// It can be returned from handlers to automatically format error responses.
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

// Error implements the error interface.
func (e *HTTPError) Error() string {
	return e.Message
}

// NewHTTPError creates a new HTTPError with the given status and message.
func NewHTTPError(status int, message string) *HTTPError {
	return &HTTPError{
		Status:  status,
		Message: message,
	}
}

// WithDetails adds optional details to the error (useful for debugging).
func (e *HTTPError) WithDetails(details any) *HTTPError {
	e.Details = details
	return e
}

// WithRequestID adds a request ID for tracing.
func (e *HTTPError) WithRequestID(id string) *HTTPError {
	e.RequestID = id
	return e
}

// SendError sends an HTTPError as a JSON response with the appropriate status code.
// Details are omitted for 5xx errors in production (security consideration).
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

// CommonErrors provides pre-built error responses for common scenarios.
var CommonErrors = struct {
	BadRequest      func(message string) *HTTPError
	Unauthorized    func() *HTTPError
	Forbidden       func() *HTTPError
	NotFound        func(resource string) *HTTPError
	Conflict        func(message string) *HTTPError
	InternalError   func(message string) *HTTPError
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
