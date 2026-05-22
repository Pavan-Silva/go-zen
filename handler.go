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

// handlerAdapter wraps a NextFunc chain as an http.Handler.
// Its only purpose is to convert the zen handler signature (func(*Context))
// to the standard net/http handler signature by acquiring a pooled Context,
// running the chain, and releasing it.
type handlerAdapter struct {
	chain NextFunc
}

// ServeHTTP implements http.Handler by acquiring a Context from the pool,
// running the middleware+handler chain, and releasing the Context.
func (a *handlerAdapter) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	c := contextPool.Get().(*Context)
	c.reset(w, r)
	a.chain(c)
	c.reset(nil, nil)
	contextPool.Put(c)
}
