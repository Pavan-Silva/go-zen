package middleware

import (
	"net/http"
	"runtime/debug"

	"github.com/Pavan-Silva/go-zen"
	"github.com/Pavan-Silva/go-zen/logger"
)

// Recover is panic recovery middleware that catches panics and sends a 500 response.
// The panic is logged with the full structural stack trace for production debugging.
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
			// Extract the stack trace bytes efficiently
			stackBytes := debug.Stack()

			// Performance Optimization: Use your zero-allocation byte-to-string utility
			// rather than forcing a heavy deep string copy on the heap.
			stackTraceStr := zen.BytesToString(stackBytes)

			// Write to the log using your updated structured attributes
			logger.Error("HTTP panic recovered",
				"error", err,
				"method", c.Request.Method,
				"path", c.Request.URL.Path,
				"remote_ip", c.Request.RemoteAddr,
				"stack", stackTraceStr,
			)

			// Framework Integration: Update context status so the Logger middleware
			// catches and reports the correct 500 Internal Server Error status code.
			c.Response.WriteHeader(http.StatusInternalServerError)

			// Safely write the standardized textual representation down the wire
			_, _ = c.Response.Write(zen.StringToBytes(http.StatusText(http.StatusInternalServerError)))
		}
	}()

	c.Next()
}
