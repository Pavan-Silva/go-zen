package middleware

import (
	"log"
	"net/http"
	"runtime/debug"

	"github.com/Pavan-Silva/go-zen"
)

// Recover is panic recovery middleware that catches panics and sends a 500 response.
// The panic is logged with the stack trace for debugging.
// Must be registered first (outermost) to catch panics from other middleware.
//
// Example:
//
//	r := zen.New(":8080")
//	r.Use(middleware.Recover)  // Must be first
//	r.Use(middleware.Logger)
//	// ... other middleware and handlers
func Recover(c *zen.Context, next http.Handler) {
	defer func() {
		if err := recover(); err != nil {
			log.Printf(
				"zen: panic recovered: %v | method=%s path=%s ip=%s\n%s",
				err,
				c.Request.Method,
				c.Request.URL.Path,
				c.Request.RemoteAddr,
				debug.Stack(),
			)
			http.Error(c.Response, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		}
	}()
	next.ServeHTTP(c.Response, c.Request)
}
