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

	"github.com/Pavan-Silva/go-zen/system"
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
func (mh *middlewareHandler) ServeHTTP(_ http.ResponseWriter, r *http.Request) {
	mh.m(FromRequest(r), mh.next)
}

// zenHandler wraps a zen-style handler (func(*Context)).
// It's registered on the mux to extract the Context from the request exactly once.
type zenHandler struct {
	fn func(*Context)
}

// ServeHTTP implements http.Handler by extracting the Context and calling the handler.
func (zh *zenHandler) ServeHTTP(_ http.ResponseWriter, r *http.Request) {
	zh.fn(FromRequest(r))
}

// Router is the main entry point for a zen application.
// It wraps http.Server and http.ServeMux, adding middleware support and Context pooling.
// The design prioritizes performance: middleware is pre-built at setup time (no per-request work),
// and Context instances are pooled to reduce GC pressure.
type RouterOption func(*Router)

func WithServer(server *http.Server) RouterOption {
	return func(r *Router) {
		r.Server = server
	}
}

func WithReadTimeout(timeout time.Duration) RouterOption {
	return func(r *Router) {
		if r.Server == nil {
			r.Server = &http.Server{}
		}
		r.Server.ReadTimeout = timeout
	}
}

func WithWriteTimeout(timeout time.Duration) RouterOption {
	return func(r *Router) {
		if r.Server == nil {
			r.Server = &http.Server{}
		}
		r.Server.WriteTimeout = timeout
	}
}

func WithIdleTimeout(timeout time.Duration) RouterOption {
	return func(r *Router) {
		if r.Server == nil {
			r.Server = &http.Server{}
		}
		r.Server.IdleTimeout = timeout
	}
}

func WithReadHeaderTimeout(timeout time.Duration) RouterOption {
	return func(r *Router) {
		if r.Server == nil {
			r.Server = &http.Server{}
		}
		r.Server.ReadHeaderTimeout = timeout
	}
}

func WithMaxHeaderBytes(bytes int) RouterOption {
	return func(r *Router) {
		if r.Server == nil {
			r.Server = &http.Server{}
		}
		r.Server.MaxHeaderBytes = bytes
	}
}

type Router struct {
	// http.Server embedded for direct access to server configuration and lifecycle.
	*http.Server
	// mux is the underlying http.ServeMux (Go 1.22+ with pattern-based routing).
	mux *http.ServeMux
	// middleware holds all globally-registered middleware functions.
	middleware []MiddlewareFunc
	// chain is the pre-built middleware chain (rebuilt on each Use() call).
	chain http.Handler
	// hasMiddleware tracks whether any middleware is registered (optimization for bypass).
	hasMiddleware bool
}

// New creates a new Router with performance-optimized and security-hardened defaults.
//
// Defaults:
// - ReadTimeout: 5 seconds (prevent slow-read attacks)
// - WriteTimeout: 10 seconds (prevent slow-write attacks)
// - IdleTimeout: 120 seconds (free resources from idle conns)
// - ReadHeaderTimeout: 2 seconds (prevent slowloris attacks)
// - MaxHeaderBytes: 1 MB (prevent header-based attacks)
//
// These timeouts are production-ready but can be customized via r.Server.
//
// Example:
//
//	r := zen.New(":8080")
//	r.Use(middleware.Logger, middleware.Recover)
//	r.Handle("GET /", homeHandler)
//	r.ListenAndServe()
func New(addr string, opts ...RouterOption) *Router {
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

	for _, opt := range opts {
		opt(r)
	}

	if r.Server == nil {
		r.Server = &http.Server{Addr: addr, Handler: r}
	} else if r.Server.Handler == nil {
		r.Server.Handler = r
	}

	return r
}

// ServeHTTP is the main entry point for every HTTP request.
// If middleware is registered, it creates a zen Context once, attaches it to the request,
// and passes it through the middleware chain. Otherwise, it delegates directly to the mux.
//
// This design ensures:
// - Context is created/destroyed exactly once per request (minimal overhead).
// - Middleware is pre-built at setup time (no per-request chain building).
// - No-middleware code path (bypass) is ultra-fast (direct mux call).
func (s *Router) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if s.hasMiddleware {
		c, r := newContext(w, r)
		defer releaseContext(c)
		s.chain.ServeHTTP(w, r)
	} else {
		s.mux.ServeHTTP(w, r)
	}
}

// Use appends middleware to the global chain and rebuilds it.
// Middleware executes in the order added.
//
// The chain is rebuilt and cached at setup time, so adding middleware has zero
// performance impact on request processing.
//
// Example:
//
//	r := zen.New(":8080")
//	r.Use(middleware.Recover)        // Must be first to catch panics
//	r.Use(middleware.Logger)
//	r.Use(middleware.CORS(...))
//	r.Use(middleware.AuthRequired)   // Can short-circuit before other handlers
func (s *Router) Use(m ...MiddlewareFunc) {
	s.middleware = append(s.middleware, m...)
	// Pre-build the chain with middleware nodes (no closures, minimal allocations).
	h := http.Handler(s.mux)
	for i := len(s.middleware) - 1; i >= 0; i-- {
		h = &middlewareHandler{m: s.middleware[i], next: h}
	}
	s.chain = h
	s.hasMiddleware = true
}

// Handle registers a zen-style handler for the given pattern.
// Patterns use Go 1.22+ format: "METHOD /path/{param}".
//
// Example:
//
//	s.Handle("GET /users", listUsers)
//	s.Handle("GET /users/{id}", getUser)
//	s.Handle("POST /users", createUser)
//	s.Handle("PUT /users/{id}", updateUser)
//	s.Handle("DELETE /users/{id}", deleteUser)
func (s *Router) Handle(pattern string, handler func(*Context)) {
	s.mux.Handle(pattern, &zenHandler{fn: handler})
}

// HandleRaw registers a standard http.Handler directly, bypassing zen Context wrapping.
// Use this for third-party handlers (e.g., pprof) or when you need standard net/http.
//
// Example:
//
//	s.HandleRaw("GET /debug/pprof/*", http.HandlerFunc(pprof.Index))
//	s.HandleRaw("GET /static/*", http.FileServer(http.Dir("static")))
func (s *Router) HandleRaw(pattern string, handler http.Handler) {
	s.mux.Handle(pattern, handler)
}

// ListenAndServe starts the HTTP server and blocks until SIGINT or SIGTERM is received.
// Then performs a graceful shutdown with a 5-second timeout.
//
// The server is started in a background goroutine so that signal handling can proceed.
// Fatal errors are logged; normal shutdown is silent.
//
// Example:
//
//	r := zen.New(":8080")
//	r.Handle("GET /", homeHandler)
//	r.ListenAndServe()  // Blocks until Ctrl+C
func (s *Router) ListenAndServe() {
	log.Print(system.Banner(s.Addr))

	go func() {
		if err := s.Server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("zen: server error: %v", err)
		}
	}()

	s.gracefulShutdown()
}

// ListenAndServeTLS starts an HTTPS server with the given certificate and key files.
// Similar to ListenAndServe, it blocks until shutdown and performs graceful shutdown.
//
// Example:
//
//	r := zen.New(":8443")
//	r.Handle("GET /", homeHandler)
//	r.ListenAndServeTLS("cert.pem", "key.pem")  // Blocks until Ctrl+C
func (s *Router) ListenAndServeTLS(certFile, keyFile string) {
	log.Print(system.Banner(s.Addr))

	go func() {
		if err := s.Server.ListenAndServeTLS(certFile, keyFile); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("zen: server error: %v", err)
		}
	}()

	s.gracefulShutdown()
}

// gracefulShutdown waits for SIGINT or SIGTERM, then shuts down the server.
// The shutdown has a 5-second timeout; contexts that don't finish in time are forced.
// Fatal errors are logged.
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
