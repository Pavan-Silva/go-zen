package auth

import (
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"fmt"
	"net/http"
	"slices"
	"strings"

	"github.com/Pavan-Silva/go-zen"
	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
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
	return slices.Contains(u.Roles, role)
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
			c.Error(http.StatusUnauthorized, http.StatusText(http.StatusUnauthorized))
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

		c.Set("user", &user)
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
			c.Error(http.StatusForbidden, http.StatusText(http.StatusForbidden))
		}
	}

	return func(c *zen.Context, next http.Handler) {
		userVal := c.Get("user")
		if userVal == nil {
			onError(c)
			return
		}

		user, ok := userVal.(*User)
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
		if user, ok := userVal.(*User); ok {
			return user
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
// Uses sorted prefix slice with binary search for O(log n) lookups.
func SkipPrefixes(prefixes ...string) SkipFunc {
	slices.Sort(prefixes)

	return func(r *http.Request) bool {
		path := r.URL.Path
		idx, found := slices.BinarySearch(prefixes, path)
		if found {
			return true
		}
		if idx > 0 && strings.HasPrefix(path, prefixes[idx-1]) {
			return true
		}
		if idx < len(prefixes) && strings.HasPrefix(path, prefixes[idx]) {
			return true
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

// ValidatePassword securely validates a plaintext password against a stored hash.
// It hashes the plain password using the same algorithm as storedHash and compares
// using constant-time comparison to prevent timing attacks.
// The storedHash should be in the format: algorithm$salt$hash (e.g., "sha256$salt$hash").
func ValidatePassword(storedHash, plain string) bool {
	if storedHash == "" || plain == "" {
		return false
	}

	before, after, ok := strings.Cut(storedHash, "$")
	if !ok {
		return false
	}

	algorithm := before
	rest := after
	parts := strings.SplitN(rest, "$", 2)
	if len(parts) != 2 {
		return false
	}

	salt := parts[0]
	existingHash := parts[1]

	newHash, err := hashPassword(algorithm, plain, salt)
	if err != nil {
		return false
	}

	return subtle.ConstantTimeCompare([]byte(existingHash), []byte(newHash)) == 1
}

// HashPassword hashes a password using the specified algorithm.
// Supported algorithms: "sha256", "bcrypt".
// For bcrypt, salt is optional as bcrypt generates its own internally.
func hashPassword(algorithm, password, salt string) (string, error) {
	switch algorithm {
	case "sha256":
		if salt == "" {
			return "", fmt.Errorf("salt is required for sha256")
		}
		data := []byte(password + salt)
		hash := make([]byte, 32)
		copy(hash, data)
		for range 10000 {
			h := sha256.Sum256(hash)
			copy(hash, h[:])
		}
		return base64.RawURLEncoding.EncodeToString(hash), nil
	case "bcrypt":
		hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
		return string(hash), err
	default:
		return "", fmt.Errorf("unsupported hashing algorithm: %s", algorithm)
	}
}

// DefaultUserMapper maps JWT claims to User struct.
// Supports standard claims: "sub" for ID, "username" for Username, "roles" or "authorities" or "scope" for Roles.
func DefaultUserMapper(claims jwt.MapClaims) User {
	user := User{
		Claims: claims,
	}

	if sub, ok := claims["sub"].(string); ok {
		user.ID = sub
	}

	if username, ok := claims["username"].(string); ok {
		user.Username = username
	} else if name, ok := claims["name"].(string); ok {
		user.Username = name
	}

	if roles, ok := claims["roles"].([]any); ok {
		user.Roles = make([]string, len(roles))
		for i, r := range roles {
			if s, ok := r.(string); ok {
				user.Roles[i] = s
			}
		}
	} else if roles, ok := claims["authorities"].([]any); ok {
		user.Roles = make([]string, len(roles))
		for i, r := range roles {
			if s, ok := r.(string); ok {
				user.Roles[i] = s
			}
		}
	} else if scope, ok := claims["scope"].(string); ok && scope != "" {
		user.Roles = strings.Split(scope, " ")
	}

	return user
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
