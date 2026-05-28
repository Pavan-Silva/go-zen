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
	return Config{ShutdownTimeout: 5 * time.Second}
}

// Engine is the top-level application container. It embeds a root RouterGroup
// and owns the http.Server and the Go 1.22+ ServeMux.
type Engine struct {
	RouterGroup
	Mux             *http.ServeMux // Mux is the underlying HTTP request multiplexer.
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

	signal.Stop(quit)
}
