package zen

import (
	"log"
	"net/http"
	"runtime/debug"
)

// Recovery catches any panic in the handler chain, logs the stack trace,
// and writes a 500 — mirroring the behaviour of Gin's Recovery() and
// Chi's Recoverer middleware.
func Recovery(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if err := recover(); err != nil {
				log.Printf("zen: panic recovered: %v\n%s", err, debug.Stack())
				http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			}
		}()
		next.ServeHTTP(w, r)
	})
}
