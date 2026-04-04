package middleware

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/Pavan-Silva/go-zen"
)

// CORSConfig holds CORS configuration options.
type CORSConfig struct {
	// AllowedOrigins list of allowed origins. Use "*" for any origin (not recommended for production).
	AllowedOrigins []string
	// AllowedMethods list of allowed HTTP methods. Default: GET, POST, PUT, DELETE, PATCH, OPTIONS.
	AllowedMethods []string
	// AllowedHeaders list of allowed request headers. Default: Content-Type, Authorization.
	AllowedHeaders []string
	// ExposeHeaders list of headers to expose to client. Default: Content-Length, Date, X-Request-ID.
	ExposeHeaders []string
	// AllowCredentials whether to allow credentials (cookies, auth headers). Default: false.
	AllowCredentials bool
	// MaxAge cache duration for preflight requests in seconds. Default: 3600.
	MaxAge int
}

// DefaultCORSConfig returns CORS config with sensible defaults.
func DefaultCORSConfig() CORSConfig {
	return CORSConfig{
		AllowedOrigins: []string{"*"},
		AllowedMethods: []string{"GET", "POST", "PUT", "DELETE", "PATCH", "OPTIONS"},
		AllowedHeaders: []string{"Content-Type", "Authorization"},
		ExposeHeaders:  []string{"Content-Length", "Date"},
		MaxAge:         3600,
	}
}

// CORS returns a middleware that handles CORS requests.
//
// Example:
//
//	r := zen.New(":8080")
//	corsConfig := middleware.DefaultCORSConfig()
//	corsConfig.AllowedOrigins = []string{"https://example.com", "https://app.example.com"}
//	r.Use(middleware.CORS(corsConfig))
func CORS(config CORSConfig) zen.MiddlewareFunc {
	return func(c *zen.Context, next http.Handler) {
		origin := c.Request.Header.Get("Origin")
		
		// Check if origin is allowed
		if !isOriginAllowed(origin, config.AllowedOrigins) {
			// Origin not allowed, but continue (don't send CORS headers)
			next.ServeHTTP(c.Response, c.Request)
			return
		}

		// Set CORS headers
		c.Response.Header().Set("Access-Control-Allow-Origin", origin)
		c.Response.Header().Set("Access-Control-Allow-Methods", strings.Join(config.AllowedMethods, ", "))
		c.Response.Header().Set("Access-Control-Allow-Headers", strings.Join(config.AllowedHeaders, ", "))
		c.Response.Header().Set("Access-Control-Expose-Headers", strings.Join(config.ExposeHeaders, ", "))
		c.Response.Header().Set("Access-Control-Max-Age", fmt.Sprintf("%d", config.MaxAge))

		if config.AllowCredentials {
			c.Response.Header().Set("Access-Control-Allow-Credentials", "true")
		}

		// Handle preflight requests
		if c.Request.Method == http.MethodOptions {
			c.Response.WriteHeader(http.StatusNoContent)
			return
		}

		next.ServeHTTP(c.Response, c.Request)
	}
}

// isOriginAllowed checks if the given origin is in the allowed list.
func isOriginAllowed(origin string, allowedOrigins []string) bool {
	if len(allowedOrigins) == 0 {
		return false
	}
	for _, allowed := range allowedOrigins {
		if allowed == "*" || allowed == origin {
			return true
		}
	}
	return false
}
