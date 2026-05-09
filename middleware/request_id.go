package middleware

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"

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
	return func(c *zen.Context, next zen.NextFunc) {
		if config.Skipper != nil && config.Skipper(c.Request) {
			next(c)
			return
		}

		requestID := c.Request.Header.Get(config.Header)
		if requestID == "" {
			if config.Generator != nil {
				requestID = config.Generator()
			} else {
				requestID = generateRequestID()
			}
		}

		c.Set("request_id", requestID)
		c.Response.Header().Set(config.Header, requestID)

		next(c)
	}
}

// generateRequestID creates a random 16-byte hex string (32 chars).
// It panics if the random number generator fails, as this indicates a serious
// system-level issue that should fail fast.
func generateRequestID() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		panic(fmt.Sprintf("failed to generate request ID: crypto/rand.Read failed: %v", err))
	}
	return hex.EncodeToString(b)
}
