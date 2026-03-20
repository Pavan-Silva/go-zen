package zen

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/Pavan-Silva/zen/zen/system"
)

// MiddlewareFunc is the zen-native middleware signature.
// Call next to pass control downstream; return without calling it to
// short-circuit (e.g. auth failure).
//
//	func Logger(c *zen.Context, next http.Handler) {
//	    log.Println(c.Request.Method, c.Request.URL.Path)
//	    next.ServeHTTP(c.Response, c.Request)
//	}
type MiddlewareFunc func(*Context, http.Handler)

// middlewareHandler is a pre-built chain node: it holds the zen middleware
// function and the already-wrapped next handler. At request time the only
// work is retrieving *Context from the request (one lookup, shared across
// the whole request) and calling the middleware function — no closures are
// allocated per request.
type middlewareHandler struct {
	m    MiddlewareFunc
	next http.Handler
}

// ServeHTTP invokes the middleware function with the request context.
// The context is guaranteed to be non-nil because [Router.ServeHTTP] always
// creates it before the chain runs.
func (mh *middlewareHandler) ServeHTTP(_ http.ResponseWriter, r *http.Request) {
	mh.m(FromRequest(r), mh.next)
}

// zenHandler wraps a zen handler function. It retrieves the Context from the
// request once and calls the handler, avoiding per-field lookups.
type zenHandler struct {
	fn func(*Context)
}

func (zh *zenHandler) ServeHTTP(_ http.ResponseWriter, r *http.Request) {
	zh.fn(FromRequest(r))
}

// Router wraps [http.Server] and its internal mux, adding middleware support
// and a zen-style handler adapter.
type Router struct {
	*http.Server
	mux           *http.ServeMux
	middleware    []MiddlewareFunc
	chain         http.Handler
	hasMiddleware bool
}

// New initialises a Router with performance-optimised and security-hardened
// defaults.
func New(addr string) *Router {
	mux := http.NewServeMux()
	r := &Router{
		mux:           mux,
		chain:         mux,
		hasMiddleware: false,
	}
	r.Server = &http.Server{
		Addr:              addr,
		Handler:           r,
		ReadTimeout:       5 * time.Second,
		WriteTimeout:      10 * time.Second,
		IdleTimeout:       120 * time.Second,
		ReadHeaderTimeout: 2 * time.Second,
		MaxHeaderBytes:    1 << 20,
	}
	return r
}

// ServeHTTP is the single entry point for every request.
// When middleware is registered, it creates the zen *Context once and attaches
// it to the request before the middleware chain runs — every middleware and
// handler reuses the same *Context with a single pointer dereference via
// FromRequest. Without middleware, it delegates directly to the mux.
func (s *Router) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if s.hasMiddleware {
		c, r := newContext(w, r)
		s.chain.ServeHTTP(w, r)
		releaseContext(c)
	} else {
		s.mux.ServeHTTP(w, r)
	}
}

// Use appends middleware to the global chain and rebuilds the cached chain.
// Middleware executes in the order added.
//
//	s.Use(middleware.Recover)
//	s.Use(middleware.Auth)
func (s *Router) Use(m ...MiddlewareFunc) {
	s.middleware = append(s.middleware, m...)
	// Build the chain using middlewareHandler nodes so no closures or
	// allocations occur at request time — only at setup time.
	h := http.Handler(s.mux)
	for i := len(s.middleware) - 1; i >= 0; i-- {
		h = &middlewareHandler{m: s.middleware[i], next: h}
	}
	s.chain = h
	s.hasMiddleware = true
}

// Handle registers a zen-style handler for the given pattern.
//
//	s.Handle("GET /users/{id}", getUser)
//	s.Handle("POST /orders",    createOrder)
func (s *Router) Handle(pattern string, handler func(*Context)) {
	s.mux.Handle(pattern, &zenHandler{fn: handler})
}

// HandleRaw registers a standard [http.Handler] directly, bypassing the zen
// Context adapter. Use for third-party handlers or pre-marshalled responses.
func (s *Router) HandleRaw(pattern string, handler http.Handler) {
	s.mux.Handle(pattern, handler)
}

// ListenAndServe starts the HTTP server and blocks until SIGINT or SIGTERM,
// then performs a graceful 5-second shutdown.
func (s *Router) ListenAndServe() {
	log.Print(system.Banner(s.Addr))

	go func() {
		if err := s.Server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("zen: server error: %v", err)
		}
	}()

	s.gracefulShutdown()
}

// ListenAndServeTLS starts the HTTPS server and shuts down gracefully.
func (s *Router) ListenAndServeTLS(certFile, keyFile string) {
	log.Print(system.Banner(s.Addr))

	go func() {
		if err := s.Server.ListenAndServeTLS(certFile, keyFile); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("zen: server error: %v", err)
		}
	}()

	s.gracefulShutdown()
}

func (s *Router) gracefulShutdown() {
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := s.Server.Shutdown(ctx); err != nil {
		log.Fatalf("zen: shutdown error: %v", err)
	}
}
