package middleware

import (
	"log"
	"net/http"
	"runtime/debug"

	"github.com/Pavan-Silva/go-zen"
)

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
			if !c.Response.(*http.ResponseWriter) == nil {
				// Check if headers already written before sending error
				// (ResponseWriter doesn't expose this, so we attempt to write)
			}
			http.Error(c.Response, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		}
	}()
	next.ServeHTTP(c.Response, c.Request)
}
