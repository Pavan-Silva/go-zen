package auth

import (
	"net/http"
	"slices"
	"time"

	"github.com/Pavan-Silva/go-zen"
)

// defaultAuthHTTPClient is the shared HTTP client used by auth providers
// (OAuth2, OIDC) when no custom client is configured.
var defaultAuthHTTPClient = &http.Client{Timeout: 10 * time.Second}

// Authenticator defines the interface for authentication logic.
// Implement this to provide custom authentication for HTTP, WebSocket, and SSE.
type Authenticator interface {
	// Authenticate validates the request and returns user info or an error.
	// For HTTP: called in middleware with the zen.Context.
	// For WS/SSE: called with the http.Request.
	Authenticate(r *http.Request) (User, error)
}

// User represents authenticated user information.
type User struct {
	ID       string
	Username string
	Roles    []string
	Claims   map[string]any
}

// HasRole checks if the user has a specific role.
func (u User) HasRole(role string) bool {
	return slices.Contains(u.Roles, role)
}

// RequireAuth creates authentication middleware.
// It stores the authenticated User in the zen.Context under the key "user".
// On failure, it sends a 401 response (customizable via WithOnError option).
// Optional skip functions bypass authentication for selected routes.
//
// Example:
//
//	r.Use(auth.RequireAuth(jwtAuth))
//	r.Use(auth.RequireAuth(jwtAuth, auth.SkipPaths("/health")))
func RequireAuth(auth Authenticator, skip ...zen.SkipFunc) func(*zen.Context, zen.NextFunc) {
	var skipper zen.SkipFunc
	if len(skip) > 0 {
		skipper = skip[0]
	}
	return MiddlewareWithSkipper(auth, nil, skipper)
}

// Middleware creates HTTP middleware that authenticates requests using the provided Authenticator.
// A custom error handler can be provided for unauthorized requests (nil sends 401).
func Middleware(auth Authenticator, onError func(*zen.Context)) func(*zen.Context, zen.NextFunc) {
	return MiddlewareWithSkipper(auth, onError, nil)
}

// MiddlewareWithSkipper creates HTTP middleware that authenticates requests and can skip selected routes.
func MiddlewareWithSkipper(auth Authenticator, onError func(*zen.Context), skip zen.SkipFunc) func(*zen.Context, zen.NextFunc) {
	if onError == nil {
		onError = func(c *zen.Context) {
			c.Error(http.StatusUnauthorized, http.StatusText(http.StatusUnauthorized))
		}
	}

	return func(c *zen.Context, next zen.NextFunc) {
		if skip != nil && skip(c.Request) {
			next(c)
			return
		}

		user, err := auth.Authenticate(c.Request)
		if err != nil {
			onError(c)
			return
		}

		c.Set("user", &user)
		next(c)
	}
}

// RequireRole creates middleware that requires a specific role.
// It must be used after RequireAuth.
func RequireRole(role string, onError func(*zen.Context)) func(*zen.Context, zen.NextFunc) {
	if onError == nil {
		onError = func(c *zen.Context) {
			c.Error(http.StatusForbidden, http.StatusText(http.StatusForbidden))
		}
	}

	return func(c *zen.Context, next zen.NextFunc) {
		userVal, ok := c.Get("user")
		if !ok || userVal == nil {
			onError(c)
			return
		}

		user, ok := userVal.(*User)
		if !ok || !user.HasRole(role) {
			onError(c)
			return
		}

		next(c)
	}
}

// GetUser retrieves the authenticated user from the zen.Context.
// Returns nil if not authenticated.
func GetUser(c *zen.Context) *User {
	if userVal, ok := c.Get("user"); ok && userVal != nil {
		if user, ok := userVal.(*User); ok {
			return user
		}
	}
	return nil
}

// WithAuth wraps an http.Handler to require authentication before serving.
// Returns 401 if authentication fails. For use with WebSocket upgrades.
func WithAuth(handler http.Handler, auth Authenticator) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if _, err := auth.Authenticate(r); err != nil {
			http.Error(w, http.StatusText(http.StatusUnauthorized), http.StatusUnauthorized)
			return
		}
		handler.ServeHTTP(w, r)
	})
}

// WithAuthFunc wraps an http.HandlerFunc to require authentication before serving.
// The authenticated User is passed to the handler. For use with WebSocket upgrades.
func WithAuthFunc(handler func(w http.ResponseWriter, r *http.Request, user User), auth Authenticator) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user, err := auth.Authenticate(r)
		if err != nil {
			http.Error(w, http.StatusText(http.StatusUnauthorized), http.StatusUnauthorized)
			return
		}
		handler(w, r, user)
	})
}
