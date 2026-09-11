package zen

import (
	"fmt"
	"net/http"

	"github.com/Pavan-Silva/go-zen/rbac"
)

// --- Authentication ---

// User represents authenticated user information.
type User struct {
	ID       string         // Unique identifier for the user.
	Username string         // Display or login name of the user.
	Roles    []string       // Roles assigned to the user; checks resolve permissions via Engine.EnableRBAC.
	Claims   map[string]any // Arbitrary claims associated with the user.
}

// Authenticator defines the interface for authentication logic.
// Implement this to provide custom authentication for HTTP, WebSocket, and SSE.
type Authenticator interface {
	// Authenticate validates the request and returns user info or an error.
	// For HTTP: called in middleware with the zen.Ctx.
	// For WS/SSE: called with the http.Request.
	Authenticate(r *http.Request) (*User, error)
}

// EnableAuth installs authentication middleware for the given authenticator,
// optionally skipping requests matched by skip funcs. Equivalent to
// r.Use(EnableAuth(authenticator, skip...)).
func (e *Engine) EnableAuth(authenticator Authenticator, skip ...SkipFunc) {
	e.Use(EnableAuth(authenticator, skip...))
}

// EnableAuth creates authentication middleware.
func EnableAuth(authenticator Authenticator, skip ...SkipFunc) HandlerFunc {
	var skipper SkipFunc
	if len(skip) > 0 {
		skipper = skip[0]
	}
	return middlewareWithSkipper(authenticator, nil, skipper)
}

// GetUser retrieves the authenticated user from the context.
// Returns nil if not authenticated.
func (c *Ctx) GetUser() *User {
	if userVal, ok := c.Get("user"); ok && userVal != nil {
		if user, ok := userVal.(*User); ok {
			return user
		}
	}
	return nil
}

// GetClaim retrieves a claim value from the authenticated user.
// Returns nil, false if no user is authenticated or the claim is missing.
func (c *Ctx) GetClaim(key string) (any, bool) {
	u := c.GetUser()
	if u == nil {
		return nil, false
	}
	val, ok := u.Claims[key]
	return val, ok
}

// middlewareWithSkipper creates HTTP middleware that authenticates requests
// and can skip selected routes.
func middlewareWithSkipper(authenticator Authenticator, onError func(*Ctx), skip SkipFunc) HandlerFunc {
	if authenticator == nil {
		panic("auth: nil Authenticator provided to MiddlewareWithSkipper")
	}

	if onError == nil {
		onError = func(c *Ctx) {
			c.Error(http.StatusUnauthorized, http.StatusText(http.StatusUnauthorized))
		}
	}

	return func(c *Ctx) {
		if skip != nil && skip(c.Request) {
			c.Next()
			return
		}

		user, err := authenticator.Authenticate(c.Request)
		if err != nil {
			onError(c)
			return
		}

		c.Set("user", user)
		c.Next()
	}
}

// --- RBAC ---

// EnableRBAC registers role definitions for authorization through the rbac
// package. Each argument is a source of roles: a string is a path to a JSON
// config file, and a rbac.Role is registered directly. With no arguments it
// loads the default rbac.DefaultConfigPath ("configs/rbac.json"):
//
//	r.EnableRBAC()                                   // loads configs/rbac.json
//	r.EnableRBAC("rbac.json")                        // custom config file
//	r.EnableRBAC(rbac.Role{Name: "admin", Permissions: []string{"users:delete"}})
//
// File roles are registered before inline ones. Panics on read or parse
// errors, on an unsupported argument type, or on more than one config path, so
// misconfiguration fails fast at startup.
func (e *Engine) EnableRBAC(sources ...any) {
	if len(sources) == 0 {
		sources = []any{rbac.DefaultConfigPath}
	}

	var (
		file   string
		inline []rbac.Role
	)
	for _, src := range sources {
		switch v := src.(type) {
		case string:
			if file != "" {
				panic(fmt.Errorf("zen: EnableRBAC: multiple config file paths: %q and %q", file, v))
			}
			if v == "" {
				v = rbac.DefaultConfigPath
			}
			file = v
		case rbac.Role:
			inline = append(inline, v)
		default:
			panic(fmt.Errorf("zen: EnableRBAC: unsupported argument type %T (want a string config path or rbac.Role)", src))
		}
	}

	if file != "" {
		if err := rbac.Apply(rbac.WithFile(file)); err != nil {
			panic(err)
		}
	}
	if len(inline) > 0 {
		if err := rbac.Apply(rbac.WithRoles(inline...)); err != nil {
			panic(err)
		}
	}
}

// --- RBAC Utility-Functions ---

// HasRole reports whether the user holds the named role. Role names match
// exactly (case-sensitive); no prefix normalization is applied.
func (u *User) HasRole(role string) bool {
	return u != nil && rbac.HasRole(u.Roles, role)
}

// HasAnyRole reports whether the user holds at least one of the roles.
func (u *User) HasAnyRole(roles ...string) bool {
	return u != nil && rbac.HasAnyRole(u.Roles, roles...)
}

// HasAllRoles reports whether the user holds every supplied role.
func (u *User) HasAllRoles(roles ...string) bool {
	return u != nil && rbac.HasAllRoles(u.Roles, roles...)
}

// HasPermission reports whether any role assigned to the user grants the
// permission, resolving through the roles registered with Engine.EnableRBAC.
// Permissions are exact, case-sensitive matches — wildcards are not
// supported. Roles that are not registered grant nothing.
func (u *User) HasPermission(permission string) bool {
	return u != nil && rbac.HasPermission(u.Roles, permission)
}

// HasAnyPermission reports whether at least one of the supplied permissions
// is granted to the user.
func (u *User) HasAnyPermission(permissions ...string) bool {
	return u != nil && rbac.HasAnyPermission(u.Roles, permissions...)
}

// HasAllPermissions reports whether every supplied permission is granted to
// the user.
func (u *User) HasAllPermissions(permissions ...string) bool {
	return u != nil && rbac.HasAllPermissions(u.Roles, permissions...)
}

// --- Authorization ---

// authorize returns middleware that denies the request with 403 via onError
// when no authenticated *User is present or check reports unauthorized.
func authorize(check func(*Ctx) bool, onError ...func(*Ctx)) HandlerFunc {
	errFunc := func(c *Ctx) {
		c.Error(http.StatusForbidden, http.StatusText(http.StatusForbidden))
	}
	if len(onError) > 0 && onError[0] != nil {
		errFunc = onError[0]
	}

	return func(c *Ctx) {
		if c.GetUser() == nil || !check(c) {
			errFunc(c)
			return
		}
		c.Next()
	}
}

// RequireRole creates middleware that requires the user to hold the named
// role. It must be used after EnableAuth. Optionally pass a custom error
// handler.
func RequireRole(role string, onError ...func(*Ctx)) HandlerFunc {
	return authorize(func(c *Ctx) bool { return c.GetUser().HasRole(role) }, onError...)
}

// RequireAnyRole creates middleware that requires at least one of the
// supplied roles. It must be used after EnableAuth.
func RequireAnyRole(roles ...string) HandlerFunc {
	return authorize(func(c *Ctx) bool { return c.GetUser().HasAnyRole(roles...) })
}

// RequireAllRoles creates middleware that requires every supplied role.
// It must be used after EnableAuth.
func RequireAllRoles(roles ...string) HandlerFunc {
	return authorize(func(c *Ctx) bool { return c.GetUser().HasAllRoles(roles...) })
}

// RequirePermission creates middleware that requires the user to be granted
// the permission through one of their registered roles (see Engine.EnableRBAC).
// It must be used after EnableAuth. Optionally pass a custom error handler.
func RequirePermission(permission string, onError ...func(*Ctx)) HandlerFunc {
	return authorize(func(c *Ctx) bool { return c.GetUser().HasPermission(permission) }, onError...)
}

// RequireAnyPermission creates middleware that requires at least one of the
// supplied permissions. It must be used after EnableAuth.
func RequireAnyPermission(permissions ...string) HandlerFunc {
	return authorize(func(c *Ctx) bool { return c.GetUser().HasAnyPermission(permissions...) })
}

// RequireAllPermissions creates middleware that requires every supplied
// permission. It must be used after EnableAuth.
func RequireAllPermissions(permissions ...string) HandlerFunc {
	return authorize(func(c *Ctx) bool { return c.GetUser().HasAllPermissions(permissions...) })
}

// RequireClaim creates middleware that requires a specific user claim value.
// It must be used after EnableAuth. Optionally pass a custom error handler.
func RequireClaim(key string, expected any, onError ...func(*Ctx)) HandlerFunc {
	return authorize(func(c *Ctx) bool {
		value, exists := c.GetClaim(key)
		return exists && equalClaims(value, expected)
	}, onError...)
}

// equalClaims compares two claim values with type coercion.
// Uses fmt.Sprint to handle JSON-decoded float64 vs int literal comparisons.
func equalClaims(a, b any) bool {
	return fmt.Sprint(a) == fmt.Sprint(b)
}

// --- Skip functions ---

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
	normalized := make([]string, 0, len(prefixes))
	for _, p := range prefixes {
		// Clean trailing slash up-front once during setup
		for len(p) > 1 && p[len(p)-1] == '/' {
			p = p[:len(p)-1]
		}
		if p != "" {
			normalized = append(normalized, p)
		}
	}

	return func(r *http.Request) bool {
		path := r.URL.Path
		pathLen := len(path)

		for _, prefix := range normalized {
			if prefix == "/" {
				return true
			}

			prefixLen := len(prefix)
			if pathLen < prefixLen {
				continue
			}

			// Slice boundary check: verifies match if exact match OR matched as a distinct route directory
			if path[:prefixLen] == prefix {
				if pathLen == prefixLen || path[prefixLen] == '/' {
					return true
				}
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
