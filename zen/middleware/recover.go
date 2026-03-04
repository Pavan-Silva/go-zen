package middleware

import (
	"log"
	"net/http"
	"runtime/debug"
)

// Recover catches any panic in the handler chain, logs the stack trace,
// and returns a 500 Internal Server Error.
func Recover(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if err := recover(); err != nil {
				// Log the error and the full stack trace for debugging.
				log.Printf("zen: panic recovered: %v\n%s", err, debug.Stack())

				// Best-effort 500 response; if headers are already written this is a no-op.
				http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			}
		}()
		next.ServeHTTP(w, r)
	})
}
