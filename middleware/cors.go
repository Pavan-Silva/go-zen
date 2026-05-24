package middleware

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/Pavan-Silva/go-zen"
)

// CORSConfig holds pre-computed configuration schemas for CORS middleware.
type CORSConfig struct {
	AllowedOrigins   []string
	AllowedMethods   []string
	AllowedHeaders   []string
	ExposeHeaders    []string
	AllowCredentials bool
	MaxAge           int

	// Internal zero-allocation lookup optimization properties
	allowedOriginsMap map[string]struct{} // Using empty struct{} saves 1 byte per key vs bool
	allowedMethodsStr string
	allowedHeadersStr string
	exposeHeadersStr  string
	maxAgeStr         string
	allowAll          bool
}

// DefaultCORSConfig returns a CORSConfig with secure production-ready defaults.
func DefaultCORSConfig() CORSConfig {
	return CORSConfig{
		AllowedOrigins: []string{}, // Locked down by default
		AllowedMethods: []string{"GET", "POST", "PUT", "DELETE", "PATCH", "HEAD", "OPTIONS"},
		AllowedHeaders: []string{"Content-Type", "Authorization", "Accept", "X-Requested-With"},
		ExposeHeaders:  []string{"Content-Length", "Date"},
		MaxAge:         3600,
	}
}

// CORS returns a high-performance framework middleware that securely isolates cross-origin resources.
func CORS(config CORSConfig) zen.HandlerFunc {
	// Pre-compute configuration states into local lexical variables during initialization
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

		// Case 1: If there's no Origin header, this isn't a cross-origin web browser request. Pass through.
		if origin == "" {
			c.Next()
			return
		}

		// Case 2: Validate origin boundaries if wildcard mode is turned off
		if !allowAll {
			if _, allowed := allowedOriginsMap[origin]; !allowed {
				// Abort early without exposing internal route layout details to rogue actors
				c.Response.WriteHeader(http.StatusForbidden)
				_, _ = c.Response.Write(zen.StringToBytes("CORS: Origin Disallowed"))
				return
			}
		}

		respHeaders := c.Response.Header()

		// --- W3C Compliance Guard Layer ---
		if allowAll {
			if config.AllowCredentials {
				// CRITICAL SECURITY COMPLIANCE: Web browsers explicitly ban mixing "*" with AllowCredentials.
				// We must echo back the incoming request origin dynamically to maintain structural validity.
				respHeaders.Set("Access-Control-Allow-Origin", origin)
				respHeaders.Set("Vary", "Origin")
			} else {
				respHeaders.Set("Access-Control-Allow-Origin", "*")
			}
		} else {
			respHeaders.Set("Access-Control-Allow-Origin", origin)
			// MANDATORY FOR CDNs: Tells intermediate proxies to isolate separate caches per origin
			respHeaders.Set("Vary", "Origin")
		}

		// Inject pre-computed string blocks down the network pipeline with zero allocation steps
		if exposeHeadersStr != "" {
			respHeaders.Set("Access-Control-Expose-Headers", exposeHeadersStr)
		}
		if config.AllowCredentials {
			respHeaders.Set("Access-Control-Allow-Credentials", "true")
		}

		// Handle Preflight Handshakes (OPTIONS)
		if c.Request.Method == http.MethodOptions {
			respHeaders.Set("Access-Control-Allow-Methods", allowedMethodsStr)
			respHeaders.Set("Access-Control-Allow-Headers", allowedHeadersStr)
			if maxAgeStr != "0" {
				respHeaders.Set("Access-Control-Max-Age", maxAgeStr)
			}

			// Return a clean 204 No Content to instantly conclude preflight checks
			c.Response.WriteHeader(http.StatusNoContent)
			return
		}

		// Continue downstream routing for standard data traffic (GET, POST, etc.)
		c.Next()
	}
}
