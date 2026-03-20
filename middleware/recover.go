package middleware

import (
	"log"
	"net/http"
	"runtime/debug"

	"github.com/Pavan-Silva/zen"
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
			http.Error(c.Response, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		}
	}()
	next.ServeHTTP(c.Response, c.Request)
}
