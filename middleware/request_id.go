package middleware

import (
	"crypto/rand"
	"encoding/hex"
	"strconv"
	"sync/atomic"
	"time"

	"github.com/Pavan-Silva/go-zen"
)

// RequestIDConfig holds configuration for RequestID middleware.
type RequestIDConfig struct {
	Header    string
	Generator func() string
	Skipper   zen.SkipFunc
}

// DefaultRequestIDConfig returns a RequestIDConfig with sensible defaults.
func DefaultRequestIDConfig() RequestIDConfig {
	return RequestIDConfig{
		Header: "X-Request-ID",
	}
}

// Global immutable machine prefix initialized once at boot time to isolate this process instance.
var nodePrefix string

// Global thread-safe atomic counter to guarantee perfect serialization sequence alignment.
var reqSequence atomic.Uint64

func init() {
	// Generate an instance token once when the module loads
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		// Fallback to time-seeded baseline if hardware crypto engine is unreachable at boot
		now := time.Now().UnixNano()
		nodePrefix = strconv.FormatUint(uint64(now), 16)
	} else {
		nodePrefix = hex.EncodeToString(b)
	}
}

// RequestID returns middleware that injects a unique request ID into each request.
func RequestID() zen.HandlerFunc {
	return RequestIDWithConfig(DefaultRequestIDConfig())
}

// RequestIDWithConfig returns RequestID middleware with optimized generation fast paths.
func RequestIDWithConfig(config RequestIDConfig) zen.HandlerFunc {
	if config.Header == "" {
		config.Header = "X-Request-ID"
	}

	return func(c *zen.Ctx) {
		if config.Skipper != nil && config.Skipper(c.Request) {
			c.Next()
			return
		}

		requestID := c.Request.Header.Get(config.Header)
		if requestID == "" {
			if config.Generator != nil {
				requestID = config.Generator()
			} else {
				// Fast Path: High-speed atomic execution block
				requestID = generateFastRequestID()
			}
		}

		// Save trace reference into Context storage layout cleanly
		c.Set("request_id", requestID)
		c.Response.Header().Set(config.Header, requestID)

		c.Next()
	}
}

// generateFastRequestID builds a highly performant traceable token by joining
// a static machine node token with a thread-safe atomic scalar variable.
func generateFastRequestID() string {
	// Atomically increase sequence counter (completely lock-free across CPU cores)
	seq := reqSequence.Add(1)

	// Pre-size the stack buffer slice allocation safely to bypass heap逃逸 (heap escape analysis)
	// Node prefix (16 bytes hex) + Separator (1 byte) + Max Uint64 text string (20 bytes)
	buf := make([]byte, 0, 40)

	buf = append(buf, nodePrefix...)
	buf = append(buf, '-')
	buf = strconv.AppendUint(buf, seq, 16) // Format sequence as ultra-compact base-16 hex value

	// Cast the stack byte slice instantly to a native string structure with zero heap copies
	return zen.BytesToString(buf)
}
