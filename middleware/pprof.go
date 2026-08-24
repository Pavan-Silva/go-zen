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
	r.GET(p+"/", wrapHandler(http.HandlerFunc(pprof.Index)))
	r.GET(p+"/cmdline", wrapHandler(http.HandlerFunc(pprof.Cmdline)))
	r.GET(p+"/profile", wrapHandler(http.HandlerFunc(pprof.Profile)))
	r.GET(p+"/symbol", wrapHandler(http.HandlerFunc(pprof.Symbol)))
	r.GET(p+"/trace", wrapHandler(http.HandlerFunc(pprof.Trace)))

	// Individual profiles are served by their dedicated pprof.Handler instead
	// of pprof.Index: Index resolves profile names by trimming a hardcoded
	// "/debug/pprof/" prefix from the request path, which breaks any custom
	// Prefix (the request would fall through to the HTML index page).
	for _, profile := range []string{"goroutine", "heap", "threadcreate", "block", "mutex", "allocs"} {
		r.GET(p+"/"+profile, wrapHandler(pprof.Handler(profile)))
	}

	// Post endpoints are required for symbol lookups during interactive CLI debugging sessions
	r.POST(p+"/symbol", wrapHandler(http.HandlerFunc(pprof.Symbol)))
}

// wrapHandler converts a standard http.Handler (including http.HandlerFunc)
// into a native zen.HandlerFunc
func wrapHandler(h http.Handler) zen.HandlerFunc {
	return func(c *zen.Ctx) {
		h.ServeHTTP(c.Response, c.Request)
	}
}
