package zen

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/Pavan-Silva/zen/zen/system"
)

// Middleware defines the function signature for zen middleware.
// It is a generic HTTP middleware, applied globally to the mux.
type Middleware func(http.Handler) http.Handler

// contextPool reuses Context objects across requests to avoid per-request heap allocations.
var contextPool = sync.Pool{
	New: func() any { return &Context{} },
}

// adaptContextHandler turns a zen-style handler into a standard http.Handler.
// It is used at route registration time and reuses Context instances via contextPool.
func adaptContextHandler(handler func(*Context)) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c := contextPool.Get().(*Context)
		c.reset(w, r)
		handler(c)
		// Clear references before returning to pool to avoid retaining request/response.
		c.Response = nil
		c.Request = nil
		c.queryCache = nil
		contextPool.Put(c)
	})
}

// Server wraps http.Server and its internal mux.
type Server struct {
	*http.Server
	mux        *http.ServeMux
	middleware []Middleware
}

// NewServer initializes a server with performance-optimized defaults.
func NewServer(addr string) *Server {
	mux := http.NewServeMux()

	s := &Server{
		mux: mux,
		Server: &http.Server{
			Addr:    addr,
			Handler: mux,
			// Performance & Security Optimizations
			ReadTimeout:       5 * time.Second,   // Max time to read the entire request
			WriteTimeout:      10 * time.Second,  // Max time to write the response
			IdleTimeout:       120 * time.Second, // Max time to keep idle connections open
			ReadHeaderTimeout: 2 * time.Second,   // Prevents Slowloris attacks
			MaxHeaderBytes:    1 << 20, // 1MB max header size
		},
	}

	return s
}

// Use adds middleware to the global chain.
// Middleware is executed in the order it is added.
func (s *Server) Use(m ...Middleware) {
	s.middleware = append(s.middleware, m...)

	// Rebuild the handler chain around the mux. This happens only at setup time,
	// never per request, so the overhead is negligible.
	h := http.Handler(s.mux)
	for i := len(s.middleware) - 1; i >= 0; i-- {
		h = s.middleware[i](h)
	}
	s.Server.Handler = h
}

// Handle registers a new handler using the zen.Context.
// The zen-style handler is adapted once into an http.Handler and attached to the mux.
func (s *Server) Handle(pattern string, handler func(*Context)) {
	s.mux.Handle(pattern, adaptContextHandler(handler))
}

// HandleHTTP registers a standard library style handler without going through
// the zen.Context adapter. This gives you net/http-level overhead for routes
// that don't need the extra helpers.
func (s *Server) HandleHTTP(pattern string, handler http.Handler) {
	s.mux.Handle(pattern, handler)
}

// --- Lifecycle Methods ---

func (s *Server) ListenAndServe() {
	fmt.Print(system.Banner(s.Addr))

	go func() {
		if err := s.Server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("zen: server error: %v", err)
		}
	}()

	s.gracefulShutdown()
}

func (s *Server) ListenAndServeTLS(certFile, keyFile string) {
	fmt.Print(system.Banner(s.Addr))

	go func() {
		if err := s.Server.ListenAndServeTLS(certFile, keyFile); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("zen: server error: %v", err)
		}
	}()

	s.gracefulShutdown()
}

func (s *Server) gracefulShutdown() {
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	// Allow 5 seconds for active connections to finish
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := s.Server.Shutdown(ctx); err != nil {
		log.Fatalf("zen: shutdown error: %v", err)
	}
}
