package middleware

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/Pavan-Silva/go-zen"
)

// CORSConfig holds CORS middleware configuration.
type CORSConfig struct {
	AllowedOrigins   []string // Origins allowed to make cross-origin requests.
	AllowedMethods   []string // HTTP methods allowed for CORS requests.
	AllowedHeaders   []string // HTTP headers allowed in CORS requests.
	ExposeHeaders    []string // Headers exposed to the client in the response.
	AllowCredentials bool     // Whether to allow credentials (cookies, auth headers).
	MaxAge           int      // Seconds the preflight result can be cached.

	allowedOriginsMap map[string]struct{}
	allowedMethodsStr string
	allowedHeadersStr string
	exposeHeadersStr  string
	maxAgeStr         string
	allowAll          bool
}

// DefaultCORSConfig returns a CORSConfig with secure defaults.
func DefaultCORSConfig() CORSConfig {
	return CORSConfig{
		AllowedOrigins: []string{}, // Locked down by default
		AllowedMethods: []string{"GET", "POST", "PUT", "DELETE", "PATCH", "HEAD", "OPTIONS"},
		AllowedHeaders: []string{"Content-Type", "Authorization", "Accept", "X-Requested-With"},
		ExposeHeaders:  []string{"Content-Length", "Date"},
		MaxAge:         3600,
	}
}

// CORS returns a CORS middleware handler.
func CORS(config CORSConfig) zen.HandlerFunc {
	// Pre-compute configuration values.
	allowedOriginsMap := make(map[string]struct{}, len(config.AllowedOrigins))
	allowAll := false

	for _, origin := range config.AllowedOrigins {
		if origin == "*" {
			allowAll = true
			break
		}
		allowedOriginsMap[origin] = struct{}{}
	}

	allowedMethodsStr := strings.Join(config.AllowedMethods, ", ")
	allowedHeadersStr := strings.Join(config.AllowedHeaders, ", ")
	exposeHeadersStr := strings.Join(config.ExposeHeaders, ", ")
	maxAgeStr := strconv.Itoa(config.MaxAge)

	return func(c *zen.Ctx) {
		origin := c.Request.Header.Get("Origin")

		// No Origin header means this is not a cross-origin request.
		if origin == "" {
			c.Next()
			return
		}

		// Check origin against allowed list.
		if !allowAll {
			if _, allowed := allowedOriginsMap[origin]; !allowed {
				// Abort with forbidden status.
				c.Response.WriteHeader(http.StatusForbidden)
				_, _ = c.Response.Write(zen.StringToBytes("CORS: Origin Disallowed"))
				return
			}
		}

		respHeaders := c.Response.Header()

		// Set CORS headers.
		if allowAll {
			if config.AllowCredentials {
				// When AllowCredentials is set, echo back the origin instead of using "*".
				respHeaders.Set("Access-Control-Allow-Origin", origin)
				respHeaders.Set("Vary", "Origin")
			} else {
				respHeaders.Set("Access-Control-Allow-Origin", "*")
			}
		} else {
			respHeaders.Set("Access-Control-Allow-Origin", origin)
			// Vary header for CDN caching.
			respHeaders.Set("Vary", "Origin")
		}

		// Set optional CORS headers.
		if exposeHeadersStr != "" {
			respHeaders.Set("Access-Control-Expose-Headers", exposeHeadersStr)
		}
		if config.AllowCredentials {
			respHeaders.Set("Access-Control-Allow-Credentials", "true")
		}

		// Handle preflight requests.
		if c.Request.Method == http.MethodOptions {
			respHeaders.Set("Access-Control-Allow-Methods", allowedMethodsStr)
			respHeaders.Set("Access-Control-Allow-Headers", allowedHeadersStr)
			if maxAgeStr != "0" {
				respHeaders.Set("Access-Control-Max-Age", maxAgeStr)
			}

			// Return 204 for preflight.
			c.Response.WriteHeader(http.StatusNoContent)
			return
		}

		// Continue to next handler.
		c.Next()
	}
}
