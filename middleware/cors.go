package middleware

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/Pavan-Silva/go-zen"
)

// CORSConfig holds configuration for CORS middleware.
// All fields have sensible defaults via DefaultCORSConfig().
//
// Note: AllowedOrigins = []string{"*"} allows any origin and is only safe for
// public APIs without authentication. For APIs with credentials, explicitly list origins.
type CORSConfig struct {
	// AllowedOrigins list of allowed origins. "*" allows any origin (insecure for private APIs).
	AllowedOrigins []string
	// allowedOriginsMap provides O(1) lookup for origin checking.
	allowedOriginsMap map[string]bool
	// AllowedMethods HTTP methods clients can use in CORS requests. Default: GET, POST, PUT, DELETE, PATCH, OPTIONS.
	AllowedMethods []string
	// AllowedHeaders request headers clients are allowed to send. Default: Content-Type, Authorization.
	AllowedHeaders []string
	// ExposeHeaders response headers the browser will expose to client JS. Default: Content-Length, Date.
	ExposeHeaders []string
	// AllowCredentials whether to allow credentials (cookies, Authorization header) in cross-origin requests. Default: false.
	AllowCredentials bool
	// MaxAge cache duration (seconds) for preflight OPTIONS responses. Default: 3600 (1 hour).
	MaxAge int
}

// DefaultCORSConfig returns a CORSConfig with secure defaults.
// Customize fields as needed for your API.
//
// Default values:
// - AllowedOrigins: [] (empty - no CORS by default, must be explicitly configured)
// - AllowedMethods: [GET, POST, PUT, DELETE, PATCH, OPTIONS]
// - AllowedHeaders: [Content-Type, Authorization]
// - ExposeHeaders: [Content-Length, Date]
// - AllowCredentials: false
// - MaxAge: 3600 seconds
//
// Example:
//
//	config := middleware.DefaultCORSConfig()
//	config.AllowedOrigins = []string{"https://example.com"}
//	config.AllowCredentials = true  // Allow cookies in cross-origin requests
//	r.Use(middleware.CORS(config))
func DefaultCORSConfig() CORSConfig {
	config := CORSConfig{
		AllowedOrigins: []string{},
		AllowedMethods: []string{"GET", "POST", "PUT", "DELETE", "PATCH", "OPTIONS"},
		AllowedHeaders: []string{"Content-Type", "Authorization"},
		ExposeHeaders:  []string{"Content-Length", "Date"},
		MaxAge:         3600,
	}
	config.allowedOriginsMap = buildOriginsMap(config.AllowedOrigins)
	return config
}

// buildOriginsMap creates a map for O(1) origin lookup.
func buildOriginsMap(origins []string) map[string]bool {
	m := make(map[string]bool, len(origins))
	for _, origin := range origins {
		m[origin] = true
	}
	return m
}

// CORS returns a middleware that handles CORS (Cross-Origin Resource Sharing) requests.
// It processes preflight OPTIONS requests and sets appropriate CORS headers.
//
// The middleware checks if the request origin is allowed. If not, it skips CORS processing
// (no Access-Control headers are set). If allowed, it:
// 1. Sets Access-Control-Allow-Origin to the request origin
// 2. Sets other Access-Control headers per config
// 3. For OPTIONS requests, responds with 204 No Content (preflight response)
// 4. For other methods, passes to the next handler
//
// Example (open to all origins - only for public APIs):
//
//	r.Use(middleware.CORS(middleware.DefaultCORSConfig()))
//
// Example (specific origins - for private APIs):
//
//	config := middleware.DefaultCORSConfig()
//	config.AllowedOrigins = []string{
//	    "https://app.example.com",
//	    "https://admin.example.com",
//	}
//	config.AllowCredentials = true
//	r.Use(middleware.CORS(config))
func CORS(config CORSConfig) zen.MiddlewareFunc {
	// Always build the origins map from current AllowedOrigins
	// This handles cases where AllowedOrigins is modified after config creation
	config.allowedOriginsMap = buildOriginsMap(config.AllowedOrigins)
	allowAll := len(config.AllowedOrigins) == 1 && config.AllowedOrigins[0] == "*"

	return func(c *zen.Context, next zen.NextFunc) {
		origin := c.Request.Header.Get("Origin")

		if origin != "" {
			allowed := allowAll || config.allowedOriginsMap[origin]
			if !allowed {
				next(c)
				return
			}
		} else if !allowAll && len(config.allowedOriginsMap) > 0 {
			next(c)
			return
		}

		allowOrigin := origin
		if !config.AllowCredentials {
			allowOrigin = "*"
		}

		c.Response.Header().Set("Access-Control-Allow-Origin", allowOrigin)
		c.Response.Header().Set("Access-Control-Allow-Methods", strings.Join(config.AllowedMethods, ", "))
		c.Response.Header().Set("Access-Control-Allow-Headers", strings.Join(config.AllowedHeaders, ", "))
		c.Response.Header().Set("Access-Control-Expose-Headers", strings.Join(config.ExposeHeaders, ", "))
		c.Response.Header().Set("Access-Control-Max-Age", fmt.Sprintf("%d", config.MaxAge))

		if config.AllowCredentials {
			c.Response.Header().Set("Access-Control-Allow-Credentials", "true")
		}

		if c.Request.Method == http.MethodOptions {
			c.Response.WriteHeader(http.StatusNoContent)
			return
		}

		next(c)
	}
}
