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
	"sync"
	"syscall"
	"time"

	"github.com/Pavan-Silva/go-zen/logger"
	"github.com/Pavan-Silva/go-zen/system"
)

// Config holds server-level timeout and header size settings.
// Use DefaultConfig to obtain sensible defaults, then override fields as needed.
type Config struct {
	ReadTimeout       time.Duration
	WriteTimeout      time.Duration
	IdleTimeout       time.Duration
	ReadHeaderTimeout time.Duration
	MaxHeaderBytes    int
	ShutdownTimeout   time.Duration
}

// DefaultConfig returns a Config with standard timeout and buffer values.
// Timeouts and MaxHeaderBytes default to 0 to bypass the Go standard library's
// tracking loops, matching raw net/http performance baselines.
func DefaultConfig() Config {
	return Config{
		ReadTimeout:       0,
		WriteTimeout:      0,
		IdleTimeout:       0,
		ReadHeaderTimeout: 0,
		MaxHeaderBytes:    0, // 0 means use net/http default baseline paths
		ShutdownTimeout:   5 * time.Second,
	}
}

// RouterGroup groups routes under a common prefix with optional shared middleware.
// An Engine embeds a root RouterGroup; Group creates child groups.
type RouterGroup struct {
	prefix     string
	engine     *Engine
	middleware []HandlerFunc
}

// Use appends middleware to the group. Middleware runs for every route registered on this group.
func (g *RouterGroup) Use(middleware ...HandlerFunc) {
	g.middleware = append(g.middleware, middleware...)
}

// Group creates a child group with an additional path prefix and optional middleware.
// The child group inherits all parent middleware and appends the new middleware after it.
// Uses a single upfront allocation to prevent double-allocation slice resizes.
func (g *RouterGroup) Group(prefix string, middleware ...HandlerFunc) *RouterGroup {
	totalLen := len(g.middleware) + len(middleware)
	combined := make([]HandlerFunc, totalLen)
	copy(combined, g.middleware)
	copy(combined[len(g.middleware):], middleware)

	return &RouterGroup{
		prefix:     g.prefix + ensureLeadingSlash(prefix),
		engine:     g.engine,
		middleware: combined,
	}
}

// Convenience methods for common HTTP methods
func (g *RouterGroup) GET(p string, h HandlerFunc)    { g.handle("GET", p, h) }
func (g *RouterGroup) POST(p string, h HandlerFunc)   { g.handle("POST", p, h) }
func (g *RouterGroup) PUT(p string, h HandlerFunc)    { g.handle("PUT", p, h) }
func (g *RouterGroup) DELETE(p string, h HandlerFunc) { g.handle("DELETE", p, h) }
func (g *RouterGroup) PATCH(p string, h HandlerFunc)  { g.handle("PATCH", p, h) }
func (g *RouterGroup) HEAD(p string, h HandlerFunc)   { g.handle("HEAD", p, h) }

// Handle parses a "METHOD /path" pattern string and registers the handler.
// If no method is present, GET is assumed.
func (g *RouterGroup) Handle(pattern string, handler HandlerFunc) {
	method, path := splitMethodPath(pattern)
	g.handle(method, path, handler)
}

// HandleWith registers a handler with per-route middleware.
// The per-route middleware runs after group middleware but before the handler.
func (g *RouterGroup) HandleWith(pattern string, handler HandlerFunc, middleware ...HandlerFunc) {
	method, path := splitMethodPath(pattern)
	g.registerRoute(method, g.prefix+path, handler, middleware)
}

// HandleRaw registers a raw http.Handler directly on the underlying ServeMux,
// bypassing the zen middleware chain for native performance.
func (g *RouterGroup) HandleRaw(pattern string, handler http.Handler) {
	method, path := splitMethodPath(pattern)
	g.engine.Mux.Handle(method+" "+g.prefix+path, handler)
}

// Prefix returns the full path prefix of this group.
func (g *RouterGroup) Prefix() string { return g.prefix }

// Core single point of entry for adding routes with group middleware.
func (g *RouterGroup) handle(method, pattern string, handler HandlerFunc) {
	g.registerRoute(method, g.prefix+pattern, handler, nil)
}

// Unified route compiler and executor block.
// Compiles the entire handler and middleware chain backwards at startup into a
// single, nested closure link. This completely eliminates slice tracking or
// index mutations within the hot request path.
func (g *RouterGroup) registerRoute(method, fullPath string, handler HandlerFunc, routeMiddleware []HandlerFunc) {
	fullPattern := method + " " + fullPath

	totalLen := len(g.middleware) + len(routeMiddleware) + 1
	chain := make([]HandlerFunc, totalLen)
	copy(chain, g.middleware)
	copy(chain[len(g.middleware):], routeMiddleware)
	chain[totalLen-1] = handler

	var finalHandler HandlerFunc = chain[len(chain)-1]

	for i := len(chain) - 2; i >= 0; i-- {
		nextHandler := finalHandler
		currentMiddleware := chain[i]

		finalHandler = func(c *Ctx) {
			c.next = nextHandler
			currentMiddleware(c)
		}
	}

	g.engine.Mux.HandleFunc(fullPattern, func(w http.ResponseWriter, r *http.Request) {
		c := g.engine.pool.Get().(*Ctx)
		c.reset(w, r)

		finalHandler(c)

		g.engine.pool.Put(c)
	})
}

// Engine is the top-level application container. It embeds a root RouterGroup
// and owns the http.Server and the Go 1.22+ ServeMux.
type Engine struct {
	RouterGroup
	Mux             *http.ServeMux
	pool            sync.Pool
	server          *http.Server
	shutdownTimeout time.Duration
}

// New creates an Engine with the given listen address and optional custom Config.
// The server's Handler is mapped directly to the ServeMux to eliminate structural router wrappers.
func New(addr string, config ...Config) *Engine {
	var cfg Config
	if len(config) > 0 {
		cfg = config[0]
	} else {
		cfg = DefaultConfig()
	}

	e := &Engine{
		Mux:             http.NewServeMux(),
		shutdownTimeout: cfg.ShutdownTimeout,
	}
	if e.shutdownTimeout <= 0 {
		e.shutdownTimeout = 5 * time.Second
	}

	e.pool.New = func() any { return &Ctx{} }
	e.RouterGroup = RouterGroup{prefix: "", engine: e, middleware: nil}

	e.server = &http.Server{
		Addr:              addr,
		Handler:           e.Mux,
		ReadTimeout:       cfg.ReadTimeout,
		WriteTimeout:      cfg.WriteTimeout,
		IdleTimeout:       cfg.IdleTimeout,
		ReadHeaderTimeout: cfg.ReadHeaderTimeout,
		MaxHeaderBytes:    cfg.MaxHeaderBytes,
	}

	return e
}

// ServeHTTP delegates directly to the underlying ServeMux.
// This supports test-suite routing when using standard engines like httptest.
func (e *Engine) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	e.Mux.ServeHTTP(w, r)
}

// File serves a single file at the given path pattern via http.ServeFile.
func (e *Engine) File(pattern, filePath string) {
	e.HandleRaw("GET "+pattern, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, filePath)
	}))
}

// Static serves static files from the given OS directory under the specified path prefix.
func (e *Engine) Static(prefix, root string) {
	e.staticFS(prefix, http.Dir(root))
}

// StaticFS serves static files from any fs.FS under the specified path prefix.
func (e *Engine) StaticFS(prefix string, filesystem fs.FS) {
	e.staticFS(prefix, http.FS(filesystem))
}

// Internal unified handler for asset serving mapping.
func (e *Engine) staticFS(prefix string, filesystem http.FileSystem) {
	prefix = strings.TrimSuffix(prefix, "/")
	if prefix == "" {
		prefix = "/"
	}

	fsHandler := http.StripPrefix(prefix, http.FileServer(filesystem))

	if prefix == "/" {
		e.HandleRaw("GET /{any...}", fsHandler)
		e.HandleRaw("HEAD /{any...}", fsHandler)
	} else {
		e.HandleRaw("GET "+prefix, fsHandler)
		e.HandleRaw("HEAD "+prefix, fsHandler)
		e.HandleRaw("GET "+prefix+"/{any...}", fsHandler)
		e.HandleRaw("HEAD "+prefix+"/{any...}", fsHandler)
	}
}

// Run starts the HTTP server and blocks until a shutdown signal is received.
func (e *Engine) Run() {
	e.runServer(e.server.ListenAndServe)
}

// RunTLS starts the HTTPS server with the given certificate and key files,
// and blocks until a shutdown signal is received.
func (e *Engine) RunTLS(certFile, keyFile string) {
	e.runServer(func() error {
		return e.server.ListenAndServeTLS(certFile, keyFile)
	})
}

// Internal execution runner for handling system interrupts and process execution loops.
func (e *Engine) runServer(listen func() error) {
	fmt.Print(system.Banner(e.server.Addr))

	go func() {
		if err := listen(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Error("Server error: %v", err)
			os.Exit(1)
		}
	}()

	e.gracefulShutdown()
}

// Tracks kernel termination signals to gracefully clean up underlying network sockets.
func (e *Engine) gracefulShutdown() {
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	ctx, cancel := context.WithTimeout(context.Background(), e.shutdownTimeout)
	defer cancel()

	if err := e.server.Shutdown(ctx); err != nil {
		logger.Error("Server shutdown error: %v", err)
	}
}

// ensureLeadingSlash sanitizes strings to guarantee an HTTP path compliance scheme.
func ensureLeadingSlash(prefix string) string {
	if prefix == "" {
		return "/"
	}
	if prefix[0] != '/' {
		return "/" + prefix
	}
	return prefix
}

// splitMethodPath cuts method headers away from raw registration keys efficiently.
func splitMethodPath(pattern string) (method, path string) {
	pattern = strings.TrimSpace(pattern)
	spaceIdx := strings.IndexByte(pattern, ' ')

	if spaceIdx == -1 {
		return "GET", ensureLeadingSlash(pattern)
	}

	method = strings.ToUpper(pattern[:spaceIdx])
	path = ensureLeadingSlash(strings.TrimSpace(pattern[spaceIdx+1:]))
	return method, path
}
