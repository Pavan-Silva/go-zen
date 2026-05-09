package zen

import (
	"net/http"
)

// NextFunc is the next handler in the middleware chain.
// Call next(c) to pass control downstream; return without calling it to
// short-circuit execution.
type NextFunc func(*Context)

// MiddlewareFunc is the zen-native middleware signature.
// Middleware receives the zen Context and the next handler in the chain.
type MiddlewareFunc func(*Context, NextFunc)

// zenHandler wraps a zen-style handler (func(*Context)).
// Registered on the mux for route matching; called directly via fn(c)
// when dispatched through the middleware chain, or via ServeHTTP (with
// FromRequest) when the mux routes to it directly (no middleware path).
type zenHandler struct {
	fn func(*Context)
}

// ServeHTTP implements http.Handler for the no-middleware path.
func (zh *zenHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	c, ok := FromRequest(r)
	if !ok {
		return
	}
	c.Response = w
	zh.fn(c)
}

// contextAwareHandler wraps a NextFunc chain with Context creation/release.
// Used by group and per-route middleware when the global chain hasn't already
// created a Context.
type contextAwareHandler struct {
	final NextFunc
}

// ServeHTTP implements http.Handler by creating a Context, running the chain, and releasing.
func (h *contextAwareHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if c, ok := FromRequest(r); ok {
		c.Response = w
		h.final(c)
		return
	}

	c, r := newContext(w, r)
	defer releaseContext(c)
	h.final(c)
}
