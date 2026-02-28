// Package zen provides a tiny micro-framework that wraps the standard
// library's http.Server and related helpers. Its goal is to reduce boilerplate
// for common operations (JSON/form parsing, graceful shutdown, etc.) without
// introducing any external dependencies or custom routing logic. The framework
// is intentionally minimal: routing is left to the net/http mux (or any
// third-party router) and zen simply adapts handlers and provides a lightweight
// Context object.

package zen

import (
    "fmt"
    "strings"
)

import (
    "context"
    "net/http"
    "os"
    "os/signal"
    "syscall"
    "time"
)

// Server wraps http.Server to provide graceful shutdown helper.
// It stays thin and just delegates to net/http.
// Use zen.NewServer to create, then call ListenAndServe or ListenAndServeTLS.

type Server struct {
    *http.Server
}

// NewServer returns a new Server with given address and handler.
func NewServer(addr string, handler http.Handler) *Server {
    return &Server{&http.Server{
        Addr:    addr,
        Handler: handler,
    }}
}

// Banner returns an ASCII art banner with a clickable link for the
// provided listen address. The returned string is safe to ignore if you don't
// want to display it; it's only intended for logging at startup and has no
// runtime cost beyond the one-time string construction.
func Banner(addr string) string {
	display := strings.TrimPrefix(addr, ":")
	if display == "" {
		display = "80"
	}

	url := "http://localhost:" + display

	// VS Code Dark+ string color (#CE9178)
	orange := "\033[38;2;206;145;120m"
	green := "\033[32m"
	gray := "\033[90m"
	bold := "\033[1m"
	reset := "\033[0m"

	art := `
   ███████╗███████╗███╗   ██╗
   ╚══███╔╝██╔════╝████╗  ██║
     ███╔╝ █████╗  ██╔██╗ ██║
    ███╔╝  ██╔══╝  ██║╚██╗██║
   ███████╗███████╗██║ ╚████║
   ╚══════╝╚══════╝╚═╝  ╚═══╝
`

	return fmt.Sprintf(`%s%s%s
  %sDeveloped by Pavan Silva%s %s(@Pavan-Silva)%s
  %sServer running at:%s %s%s%s

`,
		bold, orange, art+reset,
		bold, reset, gray, reset,
		bold, reset, green, url, reset,
	)
}   
                     
// ListenAndServe starts the server and listens for termination signals to shut down gracefully.
func (s *Server) ListenAndServe() error {
    fmt.Print(Banner(s.Addr))

    go func() {
        if err := s.Server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
            // log or ignore; user can handle
        }
    }()

    quit := make(chan os.Signal, 1)
    signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
    <-quit

    ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
    defer cancel()
    return s.Server.Shutdown(ctx)
}

// ListenAndServeTLS behaves like http.Server.ListenAndServeTLS with graceful shutdown.
func (s *Server) ListenAndServeTLS(certFile, keyFile string) error {
    fmt.Print(Banner(s.Addr))

    go func() {
        if err := s.Server.ListenAndServeTLS(certFile, keyFile); err != nil && err != http.ErrServerClosed {
            // log or ignore
        }
    }()

    quit := make(chan os.Signal, 1)
    signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
    <-quit

    ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
    defer cancel()
    return s.Server.Shutdown(ctx)
}
