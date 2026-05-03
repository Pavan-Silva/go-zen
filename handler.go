package zen

import (
	"net/http"
)

// MiddlewareFunc is the zen-native middleware signature.
// Middleware receives the zen Context and the next handler in the chain.
// Call next.ServeHTTP() to pass control downstream; return without calling it to
// short-circuit execution (useful for auth, logging, etc).
//
// Middleware is zero-allocation by design: no closures are created per request,
// and the Context is guaranteed to be non-nil due to the Router's ServeHTTP logic.
//
// Example:
//
//	func Logger(c *zen.Context, next http.Handler) {
//	    start := time.Now()
//	    log.Printf("%s %s", c.Request.Method, c.Request.URL.Path)
//	    next.ServeHTTP(c.Response, c.Request)
//	    log.Printf("took %v", time.Since(start))
//	}
//
//	func RequireAuth(c *zen.Context, next http.Handler) {
//	    token := c.Request.Header.Get("Authorization")
//	    if token == "" {
//	        c.Error(http.StatusUnauthorized, "unauthorized")
//	        return  // short-circuit: don't call next
//	    }
//	    next.ServeHTTP(c.Response, c.Request)
//	}
type MiddlewareFunc func(*Context, http.Handler)

// middlewareHandler is a chain node in the middleware stack.
// It holds a MiddlewareFunc and the next handler, written to avoid allocations.
// Per-request work is minimal: extract Context from request and call the middleware.
type middlewareHandler struct {
	m    MiddlewareFunc
	next http.Handler
}

// ServeHTTP implements http.Handler by calling the middleware function.
// The Context is guaranteed to exist because Router.ServeHTTP creates it first.
func (mh *middlewareHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	c := FromRequest(r)
	if c == nil {
		return
	}
	c.Response = w
	mh.m(c, mh.next)
}

// zenHandler wraps a zen-style handler (func(*Context)).
// It's registered on the mux to extract the Context from the request exactly once.
type zenHandler struct {
	fn func(*Context)
}

// ServeHTTP implements http.Handler by extracting the Context and calling the handler.
func (zh *zenHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	c := r.Context().Value(zenCtxKey{}).(*Context)
	c.Response = w
	zh.fn(c)
}

// contextAwareHandler wraps a middleware chain with Context creation/release.
// This handler is used when routes have group-level middleware attached.
// It ensures the Context is created once and available to all middleware/handlers,
// and is properly released back to the pool after the request.
type contextAwareHandler struct {
	chain http.Handler
}

// ServeHTTP implements http.Handler by creating a Context, running the chain, and releasing.
func (h *contextAwareHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Reuse existing Context when one is already attached (e.g. global middleware path).
	if c, _ := r.Context().Value(zenCtxKey{}).(*Context); c != nil {
		c.Response = w
		h.chain.ServeHTTP(w, r)
		return
	}

	c, r := newContext(w, r)
	defer releaseContext(c)
	h.chain.ServeHTTP(w, r)
}
