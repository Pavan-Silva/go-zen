// Package middleware provides built-in HTTP middleware for zen (recovery, logging, CORS, CSRF, compression, rate limiting, body limit, pprof).
package middleware

import (
	"net/http"
	"runtime/debug"

	"github.com/Pavan-Silva/go-zen"
	"github.com/Pavan-Silva/go-zen/internal/bytesconv"
	"github.com/Pavan-Silva/go-zen/internal/log"
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
			stack := debug.Stack()
			log.Error("HTTP panic recovered, error=%v method=%s path=%s remote_ip=%s\n%s",
				err, c.Request.Method, c.Request.URL.Path, c.Request.RemoteAddr, stack)

			c.Response.WriteHeader(http.StatusInternalServerError)

			_, _ = c.Response.Write(bytesconv.StringToBytes(http.StatusText(http.StatusInternalServerError)))
		}
	}()

	c.Next()
}
