package middleware

import (
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/Pavan-Silva/go-zen"
)

// Logger logs HTTP requests with method, path, status code, and response time.
// Includes remote IP, request size, and response size for debugging.
//
// Example:
//
//	r := zen.New(":8080")
//	r.Use(middleware.Logger)
func Logger(c *zen.Context, next http.Handler) {
	start := time.Now()
	
	// Capture the response writer to track status code and bytes written
	rw := &responseWriter{ResponseWriter: c.Response}
	
	next.ServeHTTP(rw, c.Request)
	
	duration := time.Since(start)
	log.Printf(
		"%s %s %d %v | ip=%s size=%d resp=%d bytes",
		c.Request.Method,
		c.Request.URL.Path,
		rw.status,
		duration,
		clientIP(c.Request),
		c.Request.ContentLength,
		rw.written,
	)
}

// responseWriter wraps http.ResponseWriter to capture status code and bytes written.
type responseWriter struct {
	http.ResponseWriter
	status  int
	written int
}

func (rw *responseWriter) WriteHeader(status int) {
	rw.status = status
	rw.ResponseWriter.WriteHeader(status)
}

func (rw *responseWriter) Write(b []byte) (int, error) {
	if rw.status == 0 {
		rw.status = http.StatusOK
	}
	n, err := rw.ResponseWriter.Write(b)
	rw.written += n
	return n, err
}

// clientIP extracts the client IP address, checking X-Forwarded-For header first
// (for proxies), then X-Real-IP, then falling back to RemoteAddr.
func clientIP(r *http.Request) string {
	if forwarded := r.Header.Get("X-Forwarded-For"); forwarded != "" {
		// X-Forwarded-For can contain multiple IPs; take the first
		if idx := len(forwarded); idx > 0 {
			if comma := -1; comma < len(forwarded) {
				for i := 0; i < len(forwarded); i++ {
					if forwarded[i] == ',' {
						comma = i
						break
					}
				}
				if comma > 0 {
					return forwarded[:comma]
				}
			}
		}
		return forwarded
	}
	if realIP := r.Header.Get("X-Real-IP"); realIP != "" {
		return realIP
	}
	return r.RemoteAddr
}

// RequestID adds a unique request ID to each request for tracing.
// The ID is generated as a UUID and available via context.Param("request-id")
// if you mount it as a route parameter, or via the X-Request-ID response header.
//
// Not a true middleware but can be extended to store in context:
//
//	import "github.com/google/uuid"
//	func RequestID(c *zen.Context, next http.Handler) {
//	    id := uuid.New().String()[:8]
//	    c.Set("request_id", id)
//	    c.Response.Header().Set("X-Request-ID", id)
//	    next.ServeHTTP(c.Response, c.Request)
//	}
var (
	// Placeholder for future UUID generation
	_requestIDCounter uint64
)

