package auth

import (
	"net/http"
	"slices"
	"time"

	"github.com/Pavan-Silva/go-zen"
)

// defaultAuthHTTPClient is the shared HTTP client used by auth providers
var defaultAuthHTTPClient = &http.Client{Timeout: 10 * time.Second}

// Authenticator defines the interface for authentication logic.
// Implement this to provide custom authentication for HTTP, WebSocket, and SSE.
type Authenticator interface {
	// Authenticate validates the request and returns user info or an error.
	// For HTTP: called in middleware with the zen.Ctx.
	// For WS/SSE: called with the http.Request.
	Authenticate(r *http.Request) (*User, error)
}

// User represents authenticated user information.
type User struct {
	ID       string
	Username string
	Roles    []string
	Claims   map[string]any
}

// HasRole checks if the user has a specific role.
func (u *User) HasRole(role string) bool {
	return slices.Contains(u.Roles, role)
}

// RequireAuth creates authentication middleware.
func RequireAuth(auth Authenticator, skip ...zen.SkipFunc) zen.HandlerFunc {
	var skipper zen.SkipFunc
	if len(skip) > 0 {
		skipper = skip[0]
	}
	return MiddlewareWithSkipper(auth, nil, skipper)
}

// Middleware creates HTTP middleware that authenticates requests using the provided Authenticator.
func Middleware(auth Authenticator, onError func(*zen.Ctx)) zen.HandlerFunc {
	return MiddlewareWithSkipper(auth, onError, nil)
}

// MiddlewareWithSkipper creates HTTP middleware that authenticates requests and can skip selected routes.
func MiddlewareWithSkipper(auth Authenticator, onError func(*zen.Ctx), skip zen.SkipFunc) zen.HandlerFunc {
	if auth == nil {
		panic("auth: nil Authenticator provided to MiddlewareWithSkipper")
	}

	if onError == nil {
		onError = func(c *zen.Ctx) {
			c.Error(http.StatusUnauthorized, http.StatusText(http.StatusUnauthorized))
		}
	}

	return func(c *zen.Ctx) {
		if skip != nil && skip(c.Request) {
			c.Next()
			return
		}

		user, err := auth.Authenticate(c.Request)
		if err != nil {
			onError(c)
			return
		}

		c.Set("user", user)
		c.Next()
	}
}

// RequireRole creates middleware that requires a specific role.
// It must be used after RequireAuth.
func RequireRole(role string, onError func(*zen.Ctx)) zen.HandlerFunc {
	if onError == nil {
		onError = func(c *zen.Ctx) {
			c.Error(http.StatusForbidden, http.StatusText(http.StatusForbidden))
		}
	}

	return func(c *zen.Ctx) {
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

		c.Next()
	}
}

// GetUser retrieves the authenticated user from the zen.Ctx.
// Returns nil if not authenticated.
func GetUser(c *zen.Ctx) *User {
	if userVal, ok := c.Get("user"); ok && userVal != nil {
		if user, ok := userVal.(*User); ok {
			return user
		}
	}
	return nil
}

// WithAuth wraps an http.Handler to require authentication before serving.
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
func WithAuthFunc(handler func(w http.ResponseWriter, r *http.Request, user *User), auth Authenticator) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user, err := auth.Authenticate(r)
		if err != nil {
			http.Error(w, http.StatusText(http.StatusUnauthorized), http.StatusUnauthorized)
			return
		}
		handler(w, r, user)
	})
}
