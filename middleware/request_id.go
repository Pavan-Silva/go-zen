package middleware

import (
	"crypto/rand"
	"encoding/hex"
	"net/http"

	"github.com/Pavan-Silva/go-zen"
)

// RequestIDConfig holds configuration for RequestID middleware.
type RequestIDConfig struct {
	// Header is the name of the header to use for the request ID.
	// Default: "X-Request-ID"
	Header string
	// Generator is a function that generates a unique request ID.
	// If nil, a default random hex generator is used.
	Generator func() string
	// Skipper defines a function to skip middleware.
	Skipper zen.SkipFunc
}

// DefaultRequestIDConfig returns a RequestIDConfig with sensible defaults.
func DefaultRequestIDConfig() RequestIDConfig {
	return RequestIDConfig{
		Header: "X-Request-ID",
	}
}

// RequestID returns middleware that injects a unique request ID into each request.
// The ID is stored in the Context via c.Set("request_id") and set as a response header.
// If the incoming request already has a request ID header, it is reused.
//
// Example:
//
//	r.Use(middleware.RequestID())
//
// Example with custom header:
//
//	config := middleware.DefaultRequestIDConfig()
//	config.Header = "X-Trace-ID"
//	r.Use(middleware.RequestIDWithConfig(config))
func RequestID() zen.MiddlewareFunc {
	return RequestIDWithConfig(DefaultRequestIDConfig())
}

// RequestIDWithConfig returns RequestID middleware with the given configuration.
func RequestIDWithConfig(config RequestIDConfig) zen.MiddlewareFunc {
	if config.Header == "" {
		config.Header = "X-Request-ID"
	}
	return func(c *zen.Context, next http.Handler) {
		if config.Skipper != nil && config.Skipper(c.Request) {
			next.ServeHTTP(c.Response, c.Request)
			return
		}

		// Reuse existing request ID if present
		requestID := c.Request.Header.Get(config.Header)
		if requestID == "" {
			if config.Generator != nil {
				requestID = config.Generator()
			} else {
				requestID = generateRequestID()
			}
		}

		// Store in context for downstream handlers
		c.Set("request_id", requestID)

		// Set response header
		c.Response.Header().Set(config.Header, requestID)

		next.ServeHTTP(c.Response, c.Request)
	}
}

// generateRequestID creates a random 16-byte hex string (32 chars).
func generateRequestID() string {
	b := make([]byte, 16)
	rand.Read(b)
	return hex.EncodeToString(b)
}
