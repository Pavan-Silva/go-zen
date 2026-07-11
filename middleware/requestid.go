package middleware

import (
	"crypto/rand"
	"encoding/hex"

	"github.com/Pavan-Silva/go-zen"
)

// RequestIDConfig holds configuration for the Request ID middleware.
type RequestIDConfig struct {
	// Header is the request/response header for the request ID.
	// Default: HeaderXRequestID ("X-Request-ID")
	Header string
	// Generator generates a unique request ID. Default: 16-byte hex.
	Generator func() string
	// Skipper optionally skips certain requests.
	Skipper zen.SkipFunc
}

// DefaultRequestIDConfig returns a RequestIDConfig with sensible defaults.
func DefaultRequestIDConfig() RequestIDConfig {
	return RequestIDConfig{
		Header: zen.HeaderXRequestID,
	}
}

func defaultRequestIDGenerator() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

// RequestID returns middleware that injects a request ID into the request
// context and response header. If the client sends a request ID header, it
// is used as-is; otherwise a new ID is generated.
func RequestID() zen.HandlerFunc {
	return RequestIDWithConfig(DefaultRequestIDConfig())
}

// RequestIDWithConfig returns Request ID middleware with the given config.
func RequestIDWithConfig(config RequestIDConfig) zen.HandlerFunc {
	if config.Header == "" {
		config.Header = zen.HeaderXRequestID
	}
	if config.Generator == nil {
		config.Generator = defaultRequestIDGenerator
	}

	return func(c *zen.Ctx) {
		if config.Skipper != nil && config.Skipper(c.Request) {
			c.Next()
			return
		}

		id := c.Request.Header.Get(config.Header)
		if id == "" {
			id = config.Generator()
		}

		c.Response.Header().Set(config.Header, id)
		c.Set("request_id", id)

		c.Next()
	}
}

// GetRequestID retrieves the request ID from the context store.
func GetRequestID(c *zen.Ctx) string {
	if val, ok := c.Get("request_id"); ok {
		if id, ok := val.(string); ok {
			return id
		}
	}
	return ""
}
