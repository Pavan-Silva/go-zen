package middleware

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"fmt"
	"net/http"
	"strings"

	"github.com/Pavan-Silva/go-zen"
)

// CSRFConfig holds configuration for CSRF middleware.
// All fields have sensible defaults via DefaultCSRFConfig().
type CSRFConfig struct {
	// TokenLength is the length of the generated CSRF token in bytes (before base64 encoding).
	// Default: 32
	TokenLength int
	// TokenLookup defines where to find the CSRF token in incoming requests.
	// Format: "source:name" where source is "header" or "form".
	// Default: "header:X-CSRF-Token"
	TokenLookup string
	// ContextKey is the key used to store the CSRF token in the request context.
	// Default: "csrf_token"
	ContextKey string
	// CookieName is the name of the CSRF cookie.
	// Default: "_csrf"
	CookieName string
	// CookiePath is the path of the CSRF cookie.
	// Default: "/"
	CookiePath string
	// CookieDomain is the optional domain of the CSRF cookie.
	// Default: "" (browser defaults to current domain)
	CookieDomain string
	// CookieSecure sets the Secure flag on the CSRF cookie.
	// Default: false (recommend true in production)
	CookieSecure bool
	// CookieHTTPOnly sets the HTTPOnly flag on the CSRF cookie.
	// Default: false (must be false for JS to read cookie value and set as header)
	CookieHTTPOnly bool
	// CookieSameSite sets the SameSite mode on the CSRF cookie.
	// Default: http.SameSiteLaxMode
	CookieSameSite http.SameSite
	// CookieMaxAge sets the Max-Age (in seconds) of the CSRF cookie.
	// Default: 0 (session cookie, expires when browser closes)
	CookieMaxAge int
	// CookieSet is called before setting the cookie on safe requests, allowing overrides.
	// If nil, defaults apply.
	CookieSet func(cookie *http.Cookie)
	// ErrorHandler is called when CSRF validation fails.
	// If nil, a default 403 Forbidden response is returned.
	ErrorHandler func(c *zen.Context)
	// Skipper skips CSRF processing for matching requests.
	Skipper zen.SkipFunc
}

// DefaultCSRFConfig returns a CSRFConfig with secure defaults.
// Customize fields as needed for your application.
//
// Default values:
// - TokenLength: 32
// - TokenLookup: "header:X-CSRF-Token"
// - ContextKey: "csrf_token"
// - CookieName: "_csrf"
// - CookiePath: "/"
// - CookieHTTPOnly: false (must be false for JS to read cookie)
// - CookieSameSite: http.SameSiteLaxMode
//
// Example:
//
//	config := middleware.DefaultCSRFConfig()
//	config.CookieSecure = true
//	config.CookieHTTPOnly = true
//	r.Use(middleware.CSRFWithConfig(config))
func DefaultCSRFConfig() CSRFConfig {
	return CSRFConfig{
		TokenLength:    32,
		TokenLookup:    "header:X-CSRF-Token",
		ContextKey:     "csrf_token",
		CookieName:     "_csrf",
		CookiePath:     "/",
		CookieHTTPOnly: false,
		CookieSameSite: http.SameSiteLaxMode,
	}
}

// CSRF returns CSRF middleware with default configuration.
// Uses the double-submit cookie pattern: a random token is set as a cookie on safe
// requests (GET, HEAD, OPTIONS, TRACE) and must be echoed back in a request header
// or form field on state-changing requests (POST, PUT, DELETE, PATCH).
//
// Example:
//
//	r.Use(middleware.CSRF())
func CSRF() zen.MiddlewareFunc {
	return CSRFWithConfig(DefaultCSRFConfig())
}

// CSRFWithConfig returns CSRF middleware with the given configuration.
//
// Example:
//
//	config := middleware.DefaultCSRFConfig()
//	config.CookieSecure = true
//	config.CookieHTTPOnly = true
//	config.TokenLookup = "header:X-CSRF-Token,form:_csrf"
//	r.Use(middleware.CSRFWithConfig(config))
func CSRFWithConfig(config CSRFConfig) zen.MiddlewareFunc {
	parts := strings.SplitN(config.TokenLookup, ":", 2)
	if len(parts) != 2 {
		panic(fmt.Sprintf("csrf: invalid TokenLookup %q, expected format \"source:name\" (e.g. \"header:X-CSRF-Token\")", config.TokenLookup))
	}
	lookupSource := strings.TrimSpace(parts[0])
	lookupName := strings.TrimSpace(parts[1])

	if config.CookieName == "" {
		config.CookieName = "_csrf"
	}
	if config.CookiePath == "" {
		config.CookiePath = "/"
	}
	if config.TokenLength <= 0 {
		config.TokenLength = 32
	}
	if config.ContextKey == "" {
		config.ContextKey = "csrf_token"
	}
	if config.ErrorHandler == nil {
		config.ErrorHandler = func(c *zen.Context) {
			http.Error(c.Response, http.StatusText(http.StatusForbidden), http.StatusForbidden)
		}
	}

	return func(c *zen.Context, next http.Handler) {
		if config.Skipper != nil && config.Skipper(c.Request) {
			next.ServeHTTP(c.Response, c.Request)
			return
		}

		// Get existing CSRF cookie value
		cookie, err := c.Request.Cookie(config.CookieName)
		token := ""
		if err == nil {
			token = cookie.Value
		}

		// Safe methods (RFC 7231): ensure CSRF cookie exists, always set it
		switch c.Request.Method {
		case http.MethodGet, http.MethodHead, http.MethodOptions, http.MethodTrace:
			if token == "" {
				var genErr error
				token, genErr = generateToken(config.TokenLength)
				if genErr != nil {
					config.ErrorHandler(c)
					return
				}
			}

			c.Set(config.ContextKey, token)
			setCSRFCookie(c, config, token)
			next.ServeHTTP(c.Response, c.Request)
			return
		}

		// Unsafe methods: validate CSRF token
		if token == "" {
			config.ErrorHandler(c)
			return
		}

		var requestToken string
		switch lookupSource {
		case "header":
			requestToken = c.Request.Header.Get(lookupName)
		case "form":
			requestToken = c.Request.FormValue(lookupName)
		default:
			panic(fmt.Sprintf("csrf: unsupported lookup source %q (must be \"header\" or \"form\")", lookupSource))
		}

		if requestToken == "" || subtle.ConstantTimeCompare([]byte(token), []byte(requestToken)) != 1 {
			config.ErrorHandler(c)
			return
		}

		next.ServeHTTP(c.Response, c.Request)
	}
}

// setCSRFCookie creates and sets the CSRF cookie on the response.
func setCSRFCookie(c *zen.Context, config CSRFConfig, token string) {
	cookie := &http.Cookie{
		Name:     config.CookieName,
		Value:    token,
		Path:     config.CookiePath,
		Domain:   config.CookieDomain,
		Secure:   config.CookieSecure,
		HttpOnly: config.CookieHTTPOnly,
		SameSite: config.CookieSameSite,
		MaxAge:   config.CookieMaxAge,
	}
	if config.CookieSet != nil {
		config.CookieSet(cookie)
	}
	http.SetCookie(c.Response, cookie)
}

// generateToken creates a cryptographically random token encoded in base64.
func generateToken(length int) (string, error) {
	b := make([]byte, length)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("csrf: failed to generate token: %w", err)
	}
	return base64.RawStdEncoding.EncodeToString(b), nil
}
