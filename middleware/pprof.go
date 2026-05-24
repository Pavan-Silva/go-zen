package middleware

import (
	"net/http"
	"net/http/pprof"

	"github.com/Pavan-Silva/go-zen"
)

// PprofConfig holds configuration options for mounting profiling endpoints.
type PprofConfig struct {
	// Prefix defines the base path for profiling hooks.
	// Default: "/debug/pprof"
	Prefix string
}

// DefaultPprofConfig returns a PprofConfig with sensible defaults.
func DefaultPprofConfig() PprofConfig {
	return PprofConfig{
		Prefix: "/debug/pprof",
	}
}

// RegisterPprof mounts Go's runtime profiling tools directly onto the Zen router.
// This bypasses intermediate mux wrappers, ensuring compatibility with standard
// visualization tools like `go tool pprof`.
//
// Example:
//
//	r := zen.New()
//	middleware.RegisterPprof(r)
func RegisterPprof(r *zen.Engine) {
	RegisterPprofWithConfig(r, DefaultPprofConfig())
}

// RegisterPprofWithConfig mounts the profiling toolset using a custom configuration prefix.
func RegisterPprofWithConfig(r *zen.Engine, config PprofConfig) {
	if config.Prefix == "" {
		config.Prefix = "/debug/pprof"
	}

	p := config.Prefix

	// Map structural endpoints directly to the Zen engine router.
	// Bypassing extra sub-muxes prevents route-matching and trailing-slash bugs.
	r.GET(p+"/", wrapHandler(pprof.Index))
	r.GET(p+"/cmdline", wrapHandler(pprof.Cmdline))
	r.GET(p+"/profile", wrapHandler(pprof.Profile))
	r.GET(p+"/symbol", wrapHandler(pprof.Symbol))
	r.GET(p+"/trace", wrapHandler(pprof.Trace))

	// Post endpoints are required for symbol lookups during interactive CLI debugging sessions
	r.POST(p+"/symbol", wrapHandler(pprof.Symbol))
}

// wrapHandler converts a standard http.HandlerFunc into a native zen.HandlerFunc
func wrapHandler(h http.HandlerFunc) zen.HandlerFunc {
	return func(c *zen.Ctx) {
		h(c.Response, c.Request)
	}
}
