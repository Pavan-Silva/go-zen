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
type Middleware func(func(*Context)) func(*Context)

// contextPool reuses Context objects across requests to avoid per-request heap allocations.
var contextPool = sync.Pool{
	New: func() any { return &Context{} },
}

// routeHandler holds a pre-built handler chain as a plain struct field.
// Using http.Handler instead of HandleFunc avoids a closure allocation,
// since the function pointer is stored directly rather than captured.
type routeHandler struct {
	handler func(*Context)
}

func (rh routeHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Acquire a context from the pool instead of allocating a new one each request.
	c := contextPool.Get().(*Context)
	c.reset(w, r)
	rh.handler(c)
	// Clear references before returning to pool to avoid retaining request/response.
	c.Response = nil
	c.Request = nil
	c.queryCache = nil
	contextPool.Put(c)
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

	return &Server{
		mux: mux,
		Server: &http.Server{
			Addr:    addr,
			Handler: mux,
			// Performance & Security Optimizations
			ReadTimeout:       5 * time.Second,   // Max time to read the entire request
			WriteTimeout:      10 * time.Second,  // Max time to write the response
			IdleTimeout:       120 * time.Second, // Max time to keep idle connections open
			ReadHeaderTimeout: 2 * time.Second,   // Prevents Slowloris attacks
			MaxHeaderBytes:    1 << 20,           // 1MB max header size
		},
	}
}

// Use adds middleware to the global chain.
// Middleware is executed in the order it is added.
func (s *Server) Use(m ...Middleware) {
	s.middleware = append(s.middleware, m...)
}

// Handle registers a new handler using the zen.Context.
// It wraps the zen-style handler into a routeHandler to avoid closure allocations.
func (s *Server) Handle(pattern string, handler func(*Context)) {
	// Pre-wrap the handler with middleware at startup, not at request time.
	// This ensures we aren't re-calculating the chain on every single request.
	finalHandler := handler
	for i := len(s.middleware) - 1; i >= 0; i-- {
		finalHandler = s.middleware[i](finalHandler)
	}

	// Register as a concrete struct rather than a HandleFunc closure,
	// so finalHandler is stored as a field and never escapes to the heap.
	s.mux.Handle(pattern, routeHandler{handler: finalHandler})
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
