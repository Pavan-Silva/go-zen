package zen

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"
)

// Middleware defines the function signature for zen middleware.
type Middleware func(func(*Context)) func(*Context)

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
// It wraps the zen-style handler into a standard http.HandlerFunc.
func (s *Server) Handle(pattern string, handler func(*Context)) {
	// Pre-wrap the handler with middleware at startup, not at request time.
	// This ensures we aren't re-calculating the chain on every single request.
	finalHandler := handler
	for i := len(s.middleware) - 1; i >= 0; i-- {
		finalHandler = s.middleware[i](finalHandler)
	}

	s.mux.HandleFunc(pattern, func(w http.ResponseWriter, r *http.Request) {
		// Initialize the zen context
		c := &Context{
			Response: w,
			Request:  r,
			Ctx:      r.Context(),
		}
		finalHandler(c)
	})
}

// --- Lifecycle Methods ---

func (s *Server) ListenAndServe() {
	fmt.Print(Banner(s.Addr))

	go func() {
		if err := s.Server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("zen: server error: %v", err)
		}
	}()

	s.gracefulShutdown()
}

func (s *Server) ListenAndServeTLS(certFile, keyFile string) {
	fmt.Print(Banner(s.Addr))

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

// Banner returns the ASCII art for startup.
func Banner(addr string) string {
	display := strings.TrimPrefix(addr, ":")
	if display == "" {
		display = "80"
	}

	url := "http://localhost:" + display
	orange, green, gray, bold, reset := "\033[38;2;206;145;120m", "\033[32m", "\033[90m", "\033[1m", "\033[0m"

	art := `
   ███████╗███████╗███╗   ██╗
   ╚══███╔╝██╔════╝████╗  ██║
     ███╔╝ █████╗  ██╔██╗ ██║
    ███╔╝  ██╔══╝  ██║╚██╗██║
   ███████╗███████╗██║ ╚████║
   ╚══════╝╚══════╝╚═╝  ╚═══╝`

	return fmt.Sprintf("%s%s%s%s\n  %sDeveloped by Pavan Silva%s %s(@Pavan-Silva)%s\n  %sServer running at:%s %s%s%s\n\n",
		bold, orange, art, reset, bold, reset, gray, reset, bold, reset, green, url, reset)
}
