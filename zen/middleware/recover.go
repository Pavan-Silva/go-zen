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
				// Log the error,method, path, address and stack
				log.Printf(
					"zen: panic recovered: %v | method=%s path=%s ip=%s\n%s",
					err,
					r.Method,
					r.URL.Path,
					r.RemoteAddr,
					debug.Stack(),
				)

				// Best-effort 500 response; if headers are already written this is a no-op.
				http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			}
		}()
		next.ServeHTTP(w, r)
	})
}
