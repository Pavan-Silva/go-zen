package zen

import (
	"log"
	"net/http"
	"runtime/debug"
)

// Recover catches any panic in the handler chain, logs the stack trace,
// and returns a 500 Internal Server Error.
func Recover(next func(*Context)) func(*Context) {
	return func(c *Context) {
		defer func() {
			if err := recover(); err != nil {
				// Log the error and the full stack trace for debugging
				log.Printf("zen: panic recovered: %v\n%s", err, debug.Stack())

				// Ensure we don't send headers if they've already been written
				// but generally, a panic happens before the response is committed.
				c.Response.WriteHeader(http.StatusInternalServerError)
				_, _ = c.Response.Write([]byte(http.StatusText(http.StatusInternalServerError)))
			}
		}()
		next(c)
	}
}
