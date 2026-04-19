package auth

import (
	"crypto/subtle"
	"net/http"
	"strings"

	"github.com/Pavan-Silva/go-zen"
)

// Authenticator defines the interface for authentication logic.
// Implement this to provide custom authentication for HTTP, WebSocket, and SSE.
type Authenticator interface {
	// Authenticate validates the request and returns user info or an error.
	// For HTTP: called in middleware with the zen.Context.
	// For WS/SSE: called with the http.Request.
	Authenticate(r *http.Request) (User, error)
}

// SkipFunc decides whether a request should bypass authentication.
// It receives the current request and returns true for routes to skip.
type SkipFunc func(*http.Request) bool

// User represents authenticated user information.
type User struct {
	ID       string
	Username string
	Roles    []string
	Claims   map[string]any
}

// HasRole checks if the user has a specific role.
func (u User) HasRole(role string) bool {
	for _, r := range u.Roles {
		if r == role {
			return true
		}
	}
	return false
}

// Middleware creates HTTP middleware that authenticates requests.
// It stores the authenticated User in the zen.Context under the key "user".
// On failure, it calls the onError function (defaults to sending 401).
func Middleware(auth Authenticator, onError func(*zen.Context)) func(*zen.Context, http.Handler) {
	return MiddlewareWithSkipper(auth, onError, nil)
}

// MiddlewareWithSkipper creates HTTP middleware that authenticates requests and can skip selected routes.
// If skip returns true, authentication is bypassed and the request proceeds.
func MiddlewareWithSkipper(auth Authenticator, onError func(*zen.Context), skip SkipFunc) func(*zen.Context, http.Handler) {
	if onError == nil {
		onError = func(c *zen.Context) {
			c.Error(http.StatusUnauthorized, "authentication required")
		}
	}

	return func(c *zen.Context, next http.Handler) {
		if skip != nil && skip(c.Request) {
			next.ServeHTTP(c.Response, c.Request)
			return
		}

		user, err := auth.Authenticate(c.Request)
		if err != nil {
			onError(c)
			return
		}

		c.Set("user", user)
		next.ServeHTTP(c.Response, c.Request)
	}
}

// RequireAuth is a convenience middleware that requires authentication.
// It accepts an optional SkipFunc so the same method can also skip selected routes.
func RequireAuth(auth Authenticator, skip ...SkipFunc) func(*zen.Context, http.Handler) {
	var skipper SkipFunc
	if len(skip) > 0 {
		skipper = skip[0]
	}
	return MiddlewareWithSkipper(auth, nil, skipper)
}

// RequireRole creates middleware that requires a specific role.
// It must be used after RequireAuth.
func RequireRole(role string, onError func(*zen.Context)) func(*zen.Context, http.Handler) {
	if onError == nil {
		onError = func(c *zen.Context) {
			c.Error(http.StatusForbidden, "insufficient permissions")
		}
	}

	return func(c *zen.Context, next http.Handler) {
		userVal := c.Get("user")
		if userVal == nil {
			onError(c)
			return
		}

		user, ok := userVal.(User)
		if !ok || !user.HasRole(role) {
			onError(c)
			return
		}

		next.ServeHTTP(c.Response, c.Request)
	}
}

// GetUser retrieves the authenticated user from the zen.Context.
// Returns nil if not authenticated.
func GetUser(c *zen.Context) *User {
	if userVal := c.Get("user"); userVal != nil {
		if user, ok := userVal.(User); ok {
			return &user
		}
	}
	return nil
}

// SkipPaths returns a SkipFunc that bypasses authentication for exact path matches.
func SkipPaths(paths ...string) SkipFunc {
	allowed := make(map[string]struct{}, len(paths))
	for _, p := range paths {
		allowed[p] = struct{}{}
	}
	return func(r *http.Request) bool {
		_, ok := allowed[r.URL.Path]
		return ok
	}
}

// SkipPrefixes returns a SkipFunc that bypasses authentication for matching path prefixes.
func SkipPrefixes(prefixes ...string) SkipFunc {
	return func(r *http.Request) bool {
		for _, prefix := range prefixes {
			if strings.HasPrefix(r.URL.Path, prefix) {
				return true
			}
		}
		return false
	}
}

// SkipMethodsAndPaths returns a SkipFunc that bypasses authentication for specific method/path pairs.
func SkipMethodsAndPaths(method string, paths ...string) SkipFunc {
	allowed := make(map[string]struct{}, len(paths))
	for _, p := range paths {
		allowed[method+" "+p] = struct{}{}
	}
	return func(r *http.Request) bool {
		_, ok := allowed[r.Method+" "+r.URL.Path]
		return ok
	}
}

// AuthenticateWS authenticates a WebSocket connection.
// Call this in your WebSocket handler before proceeding.
func AuthenticateWS(auth Authenticator, r *http.Request) (User, error) {
	return auth.Authenticate(r)
}

// AuthenticateSSE authenticates an SSE request.
// Call this in your SSE handler before proceeding.
func AuthenticateSSE(auth Authenticator, r *http.Request) (User, error) {
	return auth.Authenticate(r)
}

// ValidatePassword securely compares passwords using constant-time comparison.
func ValidatePassword(hashed, plain string) bool {
	return subtle.ConstantTimeCompare([]byte(hashed), []byte(plain)) == 1
}
