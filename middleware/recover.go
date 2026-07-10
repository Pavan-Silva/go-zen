// Package middleware provides built-in HTTP middleware for zen (recovery, logging, CORS, CSRF, compression, rate limiting, body limit, pprof).
package middleware

import (
	"net/http"
	"runtime/debug"

	"github.com/Pavan-Silva/go-zen"
	"github.com/Pavan-Silva/go-zen/logger"
)

// Recover catches panics, logs the stack trace, and sends a 500 response.
// Must be registered first (outermost) to catch panics from subsequent handlers.
//
// Example:
//
//	r := zen.New(":8080")
//	r.Use(middleware.Recover) // Must be first
//	r.Use(middleware.Logger)
func Recover(c *zen.Ctx) {
	defer func() {
		if err := recover(); err != nil {
			stackBytes := debug.Stack()
			stackTraceStr := zen.BytesToString(stackBytes)

			logger.Error("HTTP panic recovered",
				"error", err,
				"method", c.Request.Method,
				"path", c.Request.URL.Path,
				"remote_ip", c.Request.RemoteAddr,
				"stack", stackTraceStr,
			)

			c.Response.WriteHeader(http.StatusInternalServerError)

			_, _ = c.Response.Write(zen.StringToBytes(http.StatusText(http.StatusInternalServerError)))
		}
	}()

	c.Next()
}
