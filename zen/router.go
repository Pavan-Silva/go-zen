package zen

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/Pavan-Silva/zen/zen/system"
)

// Middleware defines the function signature for zen middleware.
// It is a generic HTTP middleware, applied globally to the mux.
type Middleware func(http.Handler) http.Handler

// Router wraps http.Server and its internal mux.
type Router struct {
	*http.Server
	mux        *http.ServeMux
	middleware []Middleware
}

// New initializes a server with performance-optimized defaults.
func New(addr string) *Router {
	mux := http.NewServeMux()

	s := &Router{
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

	return s
}

// Use adds middleware to the global chain.
// Middleware is executed in the order it is added.
func (s *Router) Use(m ...Middleware) {
	s.middleware = append(s.middleware, m...)

	// Always rebuild from the raw mux — never from s.Server.Handler — so that
	// repeated Use() calls don't double-wrap previously added middleware.
	h := http.Handler(s.mux)
	for i := len(s.middleware) - 1; i >= 0; i-- {
		h = s.middleware[i](h)
	}
	s.Server.Handler = h
}

// Handle registers a new handler using the zen.Context.
// The zen-style handler is adapted once into an http.Handler and attached to the mux.
func (s *Router) Handle(pattern string, handler func(*Context)) {
	s.mux.Handle(pattern, adaptHandler(handler))
}

// HandleRaw registers a standard library style handler without going through
// the zen.Context adapter. This gives you net/http-level overhead for routes
// that don't need the extra helpers.
func (s *Router) HandleRaw(pattern string, handler http.Handler) {
	s.mux.Handle(pattern, handler)
}

// --- Lifecycle Methods ---

func (s *Router) ListenAndServe() {
	fmt.Print(system.Banner(s.Addr))

	go func() {
		if err := s.Server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("zen: server error: %v", err)
		}
	}()

	s.gracefulShutdown()
}

func (s *Router) ListenAndServeTLS(certFile, keyFile string) {
	fmt.Print(system.Banner(s.Addr))

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

	// Allow 5 seconds for active connections to finish.
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := s.Server.Shutdown(ctx); err != nil {
		log.Fatalf("zen: shutdown error: %v", err)
	}
}
