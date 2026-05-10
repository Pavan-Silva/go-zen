package zen

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/Pavan-Silva/go-zen/logger"
	"github.com/Pavan-Silva/go-zen/system"
)

// Router is the main entry point for a zen application.
// It wraps http.Server and http.ServeMux, adding middleware support and Context pooling.
// The design prioritizes performance: middleware is pre-built at setup time (no per-request work),
// and Context instances are pooled to reduce GC pressure.

// Config holds configuration for the Router and underlying HTTP server.
type Config struct {
	// ReadTimeout is the maximum duration before timing out reads from the client.
	// Default: 5 seconds (prevent slow-read attacks).
	ReadTimeout time.Duration
	// WriteTimeout is the maximum duration before timing out writes to the client.
	// Default: 10 seconds (prevent slow-write attacks).
	WriteTimeout time.Duration
	// IdleTimeout is the maximum duration an idle connection can stay open.
	// Default: 120 seconds (free resources from idle connections).
	IdleTimeout time.Duration
	// ReadHeaderTimeout is the maximum duration for reading request headers.
	// Default: 2 seconds (prevent slowloris attacks).
	ReadHeaderTimeout time.Duration
	// MaxHeaderBytes is the maximum number of bytes the server will read parsing request headers.
	// Default: 1 MB (prevent header-based attacks).
	MaxHeaderBytes int
	// ShutdownTimeout is the maximum duration for graceful shutdown.
	// Default: 5 seconds.
	ShutdownTimeout time.Duration
	// RequestTimeout is the default per-request timeout applied to all routes.
	// When set, the request context is cancelled after this duration.
	// Default: 0 (no timeout).
	RequestTimeout time.Duration
	// Server allows replacing the http.Server entirely.
	// If nil, one will be created with the other Config values.
	Server *http.Server
}

// DefaultConfig returns a Config with production-ready secure defaults.
// These defaults are suitable for most applications; customize as needed.
func DefaultConfig() Config {
	return Config{
		ReadTimeout:       5 * time.Second,
		WriteTimeout:      10 * time.Second,
		IdleTimeout:       120 * time.Second,
		ReadHeaderTimeout: 2 * time.Second,
		MaxHeaderBytes:    1 << 20, // 1 MB
		ShutdownTimeout:   5 * time.Second,
		RequestTimeout:    0, // No default timeout
	}
}

type Router struct {
	// http.Server embedded for direct access to server configuration and lifecycle.
	*http.Server
	// mux is the underlying http.ServeMux (Go 1.22+ with pattern-based routing).
	mux *http.ServeMux
	// middleware holds all globally-registered middleware functions.
	middleware []MiddlewareFunc
	// chainNext is the pre-built middleware chain (NextFunc dispatch, no FromRequest).
	chainNext NextFunc
	// hasMiddleware tracks whether any middleware is registered (optimization for bypass).
	hasMiddleware bool
	// shutdownTimeout is the maximum duration for graceful shutdown.
	shutdownTimeout time.Duration
	// requestTimeout is the default per-request timeout.
	requestTimeout time.Duration
}

// New creates a new Router with the given address and optional configuration.
// If no config is provided, DefaultConfig() is used.
//
// Defaults:
// - ReadTimeout: 5 seconds (prevent slow-read attacks)
// - WriteTimeout: 10 seconds (prevent slow-write attacks)
// - IdleTimeout: 120 seconds (free resources from idle conns)
// - ReadHeaderTimeout: 2 seconds (prevent slowloris attacks)
// - MaxHeaderBytes: 1 MB (prevent header-based attacks)
//
// These timeouts are production-ready but can be customized via the config parameter.
//
// Example:
//
//	r := zen.New(":8080")
//
//	config := zen.DefaultConfig()
//	config.ReadTimeout = 10 * time.Second
//	r := zen.New(":8080", config)
func New(addr string, config ...Config) *Router {
	mux := http.NewServeMux()
	r := &Router{
		mux:             mux,
		hasMiddleware:   false,
		shutdownTimeout: 5 * time.Second,
		requestTimeout:  0,
	}

	// Use provided config or default
	var cfg Config
	if len(config) > 0 {
		cfg = config[0]
	} else {
		cfg = DefaultConfig()
	}
	if cfg.ShutdownTimeout > 0 {
		r.shutdownTimeout = cfg.ShutdownTimeout
	}

	// Use provided server or create new one
	if cfg.Server != nil {
		r.Server = cfg.Server
		if r.Server.Handler == nil {
			r.Server.Handler = r
		}
	} else {
		r.Server = &http.Server{
			Addr:              addr,
			Handler:           r,
			ReadTimeout:       cfg.ReadTimeout,
			WriteTimeout:      cfg.WriteTimeout,
			IdleTimeout:       cfg.IdleTimeout,
			ReadHeaderTimeout: cfg.ReadHeaderTimeout,
			MaxHeaderBytes:    cfg.MaxHeaderBytes,
		}
	}

	r.requestTimeout = cfg.RequestTimeout
	return r
}

// ServeHTTP is the main entry point for every HTTP request.
// It creates a zen Context once per request so zen handlers and middleware always
// receive a non-nil Context. The request is then passed through the middleware
// chain (if configured) or directly to the mux.
// If RequestTimeout is configured, the request context is wrapped with a timeout.
func (s *Router) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	c, r := newContext(w, r)
	defer releaseContext(c)

	if s.requestTimeout > 0 {
		ctx, cancel := context.WithTimeout(r.Context(), s.requestTimeout)
		defer cancel()
		r = r.WithContext(ctx)
	}

	if s.hasMiddleware {
		c.Response = w
		c.Request = r
		s.chainNext(c)
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
	s.chainNext = s.buildChain()
	s.hasMiddleware = true
}

// buildChain creates the NextFunc chain from registered middleware.
// The chain passes *Context directly — no FromRequest lookups, no per-request allocations.
func (s *Router) buildChain() NextFunc {
	if len(s.middleware) == 0 {
		return nil
	}
	var tail NextFunc
	tail = func(c *Context) {
		handler, _ := s.mux.Handler(c.Request)
		if zh, ok := handler.(*zenHandler); ok {
			zh.fn(c)
		} else {
			handler.ServeHTTP(c.Response, c.Request)
		}
	}
	for i := len(s.middleware) - 1; i >= 0; i-- {
		mw := s.middleware[i]
		prev := tail
		tail = func(c *Context) {
			mw(c, prev)
		}
	}
	return tail
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

// HandleWith registers a zen-style handler with per-route middleware.
// The middleware only applies to this specific route, not globally.
// Middleware executes after global middleware but before the handler.
//
// Example:
//
//	s.HandleWith("GET /admin", adminHandler, authMiddleware, auditLog)
//	s.HandleWith("POST /users", createUser, rateLimitMiddleware)
func (s *Router) HandleWith(pattern string, handler func(*Context), middleware ...MiddlewareFunc) {
	s.handleWithMiddleware(pattern, handler, middleware)
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

// File serves a single file for the given route pattern.
// The path may be a full filesystem path or relative file path.
//
// Example:
//
//	r.File("GET /", "public/index.html")
func (s *Router) File(pattern, filePath string) {
	h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, filePath)
	})
	s.HandleRaw(pattern, h)
}

// Static serves files from a local directory under the given URL prefix.
// It uses http.FileServer and strips the prefix from the request path.
//
// Example:
//
//	r.Static("/images", "assets/images")
func (s *Router) Static(prefix, root string) {
	prefix = strings.TrimSuffix(prefix, "/")
	if prefix == "" {
		prefix = "/"
	}
	fsHandler := http.StripPrefix(prefix, http.FileServer(http.Dir(root)))
	if prefix == "/" {
		s.HandleRaw("GET /", fsHandler)
		s.HandleRaw("HEAD /", fsHandler)
	} else {
		s.HandleRaw("GET "+prefix, fsHandler)
		s.HandleRaw("HEAD "+prefix, fsHandler)
		s.HandleRaw("GET "+prefix+"/", fsHandler)
		s.HandleRaw("HEAD "+prefix+"/", fsHandler)
	}
}

// StaticFS serves files from an fs.FS under the given URL prefix.
// This supports embedded file systems (embed.FS) and custom fs.FS implementations.
//
// Example:
//
//	imagesFS, _ := fs.Sub(images, "assets/images")
//	r.StaticFS("/images", imagesFS)
func (s *Router) StaticFS(prefix string, filesystem fs.FS) {
	prefix = strings.TrimSuffix(prefix, "/")
	if prefix == "" {
		prefix = "/"
	}
	fsHandler := http.StripPrefix(prefix, http.FileServer(http.FS(filesystem)))
	if prefix == "/" {
		s.HandleRaw("GET /", fsHandler)
		s.HandleRaw("HEAD /", fsHandler)
	} else {
		s.HandleRaw("GET "+prefix, fsHandler)
		s.HandleRaw("HEAD "+prefix, fsHandler)
		s.HandleRaw("GET "+prefix+"/", fsHandler)
		s.HandleRaw("HEAD "+prefix+"/", fsHandler)
	}
}

// Run starts the HTTP server and blocks until SIGINT or SIGTERM is received.
// Then performs a graceful shutdown with a configurable timeout.
//
// The server is started in a background goroutine so that signal handling can proceed.
// Fatal errors are logged; normal shutdown is silent.
//
// Example:
//
//	r := zen.New(":8080")
//	r.Handle("GET /", homeHandler)
//	r.Run()  // Blocks until Ctrl+C
func (s *Router) Run() {
	s.runServer(s.Server.ListenAndServe)
}

// RunTLS starts an HTTPS server with the given certificate and key files.
// Similar to Run, it blocks until shutdown and performs graceful shutdown.
//
// Example:
//
//	r := zen.New(":8443")
//	r.Handle("GET /", homeHandler)
//	r.RunTLS("cert.pem", "key.pem")  // Blocks until Ctrl+C
func (s *Router) RunTLS(certFile, keyFile string) {
	s.runServer(func() error {
		return s.Server.ListenAndServeTLS(certFile, keyFile)
	})
}

// runServer starts the server and handles graceful shutdown.
func (s *Router) runServer(listen func() error) {
	addr := s.Addr
	if addr == "" && s.Server != nil {
		addr = s.Server.Addr
	}

	fmt.Print(system.Banner(addr))

	go func() {
		if err := listen(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Fatal("Server: error: %v", err)
		}
	}()

	s.gracefulShutdown()
}

// gracefulShutdown waits for SIGINT or SIGTERM, then shuts down the server.
// The shutdown timeout is configurable via Config.ShutdownTimeout.
func (s *Router) gracefulShutdown() {
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	ctx, cancel := context.WithTimeout(context.Background(), s.shutdownTimeout)
	defer cancel()

	if err := s.Server.Shutdown(ctx); err != nil {
		logger.Fatal("Server: shutdown error: %v", err)
	}
}
