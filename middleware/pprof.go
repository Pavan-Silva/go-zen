package middleware

import (
	"net/http"
	"net/http/pprof"
)

// PprofConfig holds configuration for pprof middleware.
type PprofConfig struct {
	// Prefix is the URL prefix for pprof endpoints.
	// Default: "/debug/pprof"
	Prefix string
}

// DefaultPprofConfig returns a PprofConfig with sensible defaults.
func DefaultPprofConfig() PprofConfig {
	return PprofConfig{
		Prefix: "/debug/pprof",
	}
}

// PprofHandler returns an http.Handler that serves pprof endpoints under the given prefix.
// This is a one-line way to mount pprof for profiling and debugging.
//
// Example:
//
//	r.HandleRaw("GET /debug/pprof/*", middleware.PprofHandler())
func PprofHandler() http.Handler {
	return PprofHandlerWithConfig(DefaultPprofConfig())
}

// PprofHandlerWithConfig returns a pprof handler with the given configuration.
func PprofHandlerWithConfig(config PprofConfig) http.Handler {
	if config.Prefix == "" {
		config.Prefix = "/debug/pprof"
	}

	mux := http.NewServeMux()
	prefix := config.Prefix

	mux.HandleFunc(prefix+"/", pprof.Index)
	mux.HandleFunc(prefix+"/cmdline", pprof.Cmdline)
	mux.HandleFunc(prefix+"/profile", pprof.Profile)
	mux.HandleFunc(prefix+"/symbol", pprof.Symbol)
	mux.HandleFunc(prefix+"/trace", pprof.Trace)

	return mux
}
