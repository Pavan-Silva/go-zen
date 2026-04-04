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

// responseWriter wraps http.ResponseWriter to capture status code and response size.
// This allows Logger to report metrics after the handler completes.
type responseWriter struct {
	http.ResponseWriter
	status  int
	written int
}

// WriteHeader records the HTTP status code.
func (rw *responseWriter) WriteHeader(status int) {
	rw.status = status
	rw.ResponseWriter.WriteHeader(status)
}

// Write records bytes written and delegates to the underlying writer.
func (rw *responseWriter) Write(b []byte) (int, error) {
	if rw.status == 0 {
		rw.status = http.StatusOK
	}
	n, err := rw.ResponseWriter.Write(b)
	rw.written += n
	return n, err
}

// clientIP extracts the client IP address for logging.
// Checks X-Forwarded-For (for reverse proxies), X-Real-IP, then RemoteAddr.
// For X-Forwarded-For with multiple IPs, returns the first (original client).
func clientIP(r *http.Request) string {
	if forwarded := r.Header.Get("X-Forwarded-For"); forwarded != "" {
		// X-Forwarded-For format: "client, proxy1, proxy2"
		// Take the first IP (original client)
		for i := 0; i < len(forwarded); i++ {
			if forwarded[i] == ',' {
				return forwarded[:i]
			}
		}
		return forwarded
	}
	if realIP := r.Header.Get("X-Real-IP"); realIP != "" {
		return realIP
	}
	return r.RemoteAddr
}

