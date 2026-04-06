package auth

import (
	"crypto/subtle"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/Pavan-Silva/go-zen"
	"github.com/golang-jwt/jwt/v5"
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

// AuthMiddleware creates HTTP middleware that authenticates requests.
// It stores the authenticated User in the zen.Context under the key "user".
// On failure, it calls the onError function (defaults to sending 401).
func AuthMiddleware(auth Authenticator, onError func(*zen.Context)) func(*zen.Context, http.Handler) {
	return AuthMiddlewareWithSkipper(auth, onError, nil)
}

// AuthMiddlewareWithSkipper creates HTTP middleware that authenticates requests and can skip selected routes.
// If skip returns true, authentication is bypassed and the request proceeds.
func AuthMiddlewareWithSkipper(auth Authenticator, onError func(*zen.Context), skip SkipFunc) func(*zen.Context, http.Handler) {
	if onError == nil {
		onError = func(c *zen.Context) {
			c.SendError(zen.Unauthorized())
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
	return AuthMiddlewareWithSkipper(auth, nil, skipper)
}

// RequireRole creates middleware that requires a specific role.
// It must be used after RequireAuth.
func RequireRole(role string, onError func(*zen.Context)) func(*zen.Context, http.Handler) {
	if onError == nil {
		onError = func(c *zen.Context) {
			c.SendError(zen.Forbidden())
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

// BasicAuth implements HTTP Basic Authentication.
type BasicAuth struct {
	Realm    string
	Validate func(username, password string) (User, error)
}

func (b *BasicAuth) Authenticate(r *http.Request) (User, error) {
	username, password, ok := r.BasicAuth()
	if !ok {
		return User{}, errors.New("missing basic auth")
	}

	return b.Validate(username, password)
}

// JWTAuth implements JWT token authentication.
type JWTAuth struct {
	Secret     []byte
	SigningMethod jwt.SigningMethod
	ClaimsFunc func(claims jwt.MapClaims) User
}

func (j *JWTAuth) Authenticate(r *http.Request) (User, error) {
	authHeader := r.Header.Get("Authorization")
	if authHeader == "" {
		return User{}, errors.New("missing authorization header")
	}

	tokenString := strings.TrimPrefix(authHeader, "Bearer ")
	if tokenString == authHeader {
		return User{}, errors.New("invalid authorization header")
	}

	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		if token.Method != j.SigningMethod {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return j.Secret, nil
	})

	if err != nil {
		return User{}, err
	}

	if claims, ok := token.Claims.(jwt.MapClaims); ok && token.Valid {
		return j.ClaimsFunc(claims), nil
	}

	return User{}, errors.New("invalid token")
}

// APIKeyAuth implements API key authentication via header or query param.
type APIKeyAuth struct {
	HeaderName string // e.g., "X-API-Key"
	QueryParam string // e.g., "api_key"
	Validate   func(key string) (User, error)
}

func (a *APIKeyAuth) Authenticate(r *http.Request) (User, error) {
	var key string

	if a.HeaderName != "" {
		key = r.Header.Get(a.HeaderName)
	}

	if key == "" && a.QueryParam != "" {
		key = r.URL.Query().Get(a.QueryParam)
	}

	if key == "" {
		return User{}, errors.New("missing API key")
	}

	return a.Validate(key)
}

// SessionAuth implements session-based authentication using cookies.
type SessionAuth struct {
	CookieName string
	Store      SessionStore
}

type SessionStore interface {
	Get(sessionID string) (User, error)
}

func (s *SessionAuth) Authenticate(r *http.Request) (User, error) {
	cookie, err := r.Cookie(s.CookieName)
	if err != nil {
		return User{}, errors.New("missing session cookie")
	}

	return s.Store.Get(cookie.Value)
}

// InMemorySessionStore is a simple in-memory session store for development.
type InMemorySessionStore struct {
	sessions map[string]User
}

func NewInMemorySessionStore() *InMemorySessionStore {
	return &InMemorySessionStore{
		sessions: make(map[string]User),
	}
}

func (s *InMemorySessionStore) Get(sessionID string) (User, error) {
	user, ok := s.sessions[sessionID]
	if !ok {
		return User{}, errors.New("invalid session")
	}
	return user, nil
}

func (s *InMemorySessionStore) Set(sessionID string, user User) {
	s.sessions[sessionID] = user
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

// GenerateJWT creates a JWT token with the given claims.
func GenerateJWT(secret []byte, method jwt.SigningMethod, claims jwt.MapClaims, expiry time.Duration) (string, error) {
	now := time.Now()
	claims["iat"] = now.Unix()
	claims["exp"] = now.Add(expiry).Unix()

	token := jwt.NewWithClaims(method, claims)
	return token.SignedString(secret)
}

// ValidatePassword securely compares passwords using constant-time comparison.
func ValidatePassword(hashed, plain string) bool {
	return subtle.ConstantTimeCompare([]byte(hashed), []byte(plain)) == 1
}
