// Package auth provides authentication and authorization middleware (JWT, OAuth2, OIDC, Basic, RBAC, ABAC, PBAC, session).
package auth

import (
	"net/http"
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
	ID          string         // Unique identifier for the user.
	Username    string         // Display or login name of the user.
	Authorities []string       // Authorities assigned to the user for authorization.
	Claims      map[string]any // Arbitrary claims associated with the user.
}

// RequireAuth creates authentication middleware.
func RequireAuth(auth Authenticator, skip ...zen.SkipFunc) zen.HandlerFunc {
	var skipper zen.SkipFunc
	if len(skip) > 0 {
		skipper = skip[0]
	}
	return MiddlewareWithSkipper(auth, nil, skipper)
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

// authorize returns middleware that denies the request with 403 via onError
// when no authenticated *User is present or check reports unauthorized.
func authorize(check func(*User) bool, onError ...func(*zen.Ctx)) zen.HandlerFunc {
	errFunc := func(c *zen.Ctx) {
		c.Error(http.StatusForbidden, http.StatusText(http.StatusForbidden))
	}
	if len(onError) > 0 && onError[0] != nil {
		errFunc = onError[0]
	}

	return func(c *zen.Ctx) {
		if user := GetUser(c); user == nil || !check(user) {
			errFunc(c)
			return
		}
		c.Next()
	}
}
