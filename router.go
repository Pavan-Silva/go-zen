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

	"github.com/Pavan-Silva/go-zen/internal/log"
	"github.com/Pavan-Silva/go-zen/internal/system"
)

var _ http.Handler = (*Engine)(nil)

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
// and owns the radix tree router and the http.Server.
type Engine struct {
	RouterGroup
	router             *radixRouter
	pool               sync.Pool
	server             *http.Server
	shutdownTimeout    time.Duration
	validator          Validator
	autoValidate       bool
	JSONSerializer     JSONSerializer
	XMLSerializer      XMLSerializer
	MaxMultipartMemory int64
}

// New creates an Engine with the given listen address and optional custom Config.
func New(addr string, config ...Config) *Engine {
	var cfg Config
	if len(config) > 0 {
		cfg = config[0]
	} else {
		cfg = DefaultConfig()
	}

	e := &Engine{
		router:             newRadixRouter(),
		shutdownTimeout:    cfg.ShutdownTimeout,
		JSONSerializer:     jsonSerializer{},
		XMLSerializer:      xmlSerializer{},
		MaxMultipartMemory: 32 << 20,
	}

	e.validator = Validator(&defaultValidate{inst: newValidator()})

	if e.shutdownTimeout == 0 {
		e.shutdownTimeout = 5 * time.Second
	}

	e.pool.New = func() any {
		return &Ctx{
			ps:           make(params, 0, 8),
			skippedNodes: make([]skippedNode, 0, 16),
		}
	}
	e.RouterGroup = RouterGroup{prefix: "", engine: e, middleware: nil}

	e.server = &http.Server{
		Addr:              addr,
		Handler:           e,
		ReadTimeout:       cfg.ReadTimeout,
		WriteTimeout:      cfg.WriteTimeout,
		IdleTimeout:       cfg.IdleTimeout,
		ReadHeaderTimeout: cfg.ReadHeaderTimeout,
		MaxHeaderBytes:    cfg.MaxHeaderBytes,
	}

	return e
}

// ServeHTTP implements the http.Handler interface using the radix tree router.
func (e *Engine) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	c := e.pool.Get().(*Ctx)
	c.reset(w, r, e)

	path := r.URL.Path
	if r.URL.RawPath != "" {
		path = r.URL.RawPath
	}

	result := e.router.find(r.Method, path, &c.ps, &c.skippedNodes)

	if result.handlers != nil {
		result.handlers[0](c)
	} else if result.tsr {
		path = r.URL.Path
		if path[len(path)-1] == '/' {
			path = path[:len(path)-1]
		} else {
			path = path + "/"
		}
		http.Redirect(w, r, path, http.StatusMovedPermanently)
	} else if result.allowedMethod != "" {
		w.Header().Set("Allow", result.allowedMethod)
		http.Error(w, "405 method not allowed\n", http.StatusMethodNotAllowed)
	} else {
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte("404 page not found\n"))
	}

	e.pool.Put(c)
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

	errCh := make(chan error, 1)
	go func() {
		errCh <- listen()
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	defer signal.Stop(quit)

	select {
	case err := <-errCh:
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Error("Server error: %v", err)
		}
		return
	case <-quit:
	}

	e.gracefulShutdown()
}

// gracefulShutdown shuts down the HTTP server with the configured timeout.
// Called after a signal is received by runServer.
func (e *Engine) gracefulShutdown() {
	ctx, cancel := context.WithTimeout(context.Background(), e.shutdownTimeout)
	defer cancel()

	if err := e.server.Shutdown(ctx); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Error("Server shutdown error: %v", err)
	}
}
