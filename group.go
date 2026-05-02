package zen

import (
	"net/http"
	"strings"
)

// ensureLeadingSlash ensures the prefix starts with a slash.
func ensureLeadingSlash(prefix string) string {
	if prefix != "" && !strings.HasPrefix(prefix, "/") {
		return "/" + prefix
	}
	return prefix
}

// combineMiddleware combines two middleware slices into a new slice.
// Used to merge group middleware with per-route or subgroup middleware.
func combineMiddleware(mw1, mw2 []MiddlewareFunc) []MiddlewareFunc {
	combined := make([]MiddlewareFunc, 0, len(mw1)+len(mw2))
	combined = append(combined, mw1...)
	combined = append(combined, mw2...)
	return combined
}

// Group represents a collection of routes with a common prefix and optional middleware.
// Route groups are the primary way to organize routes and apply middleware to subsets
// of your API.
//
// Example:
//
//	r := zen.New(":8080")
//
//	// Public routes - no auth required
//	api := r.Group("/api")
//	api.Handle("GET /health", healthCheck)
//
//	// Protected routes - auth middleware applied
//	users := r.Group("/api/users", authMiddleware)
//	users.Handle("GET /{id}", getUser)
//	users.Handle("PUT /{id}", updateUser)
//
//	// Nested groups
//	admin := r.Group("/admin", adminMiddleware)
//	admin.Handle("GET /dashboard", dashboard)
//
//	// Sub-groups inherit parent middleware
//	adminUsers := admin.Group("/users")
//	adminUsers.Handle("GET /{id}", adminGetUser)
type Group struct {
	router     *Router
	prefix     string
	middleware []MiddlewareFunc
}

// Group creates a new route group with the given prefix and optional middleware.
// The prefix is prepended to all routes registered through this group.
// Middleware applied to the group runs before handlers in that group.
//
// The provided prefix should start with a leading slash (e.g., "/api/v1").
// If the prefix doesn't start with a slash, one will be prepended.
//
// Example:
//
//	users := r.Group("/api/users", authMiddleware)
//	users.Handle("GET /{id}", getUser)
//	users.Handle("PUT /{id}", updateUser)
//
//	// Nested groups
//	admin := r.Group("/admin", adminMiddleware)
//	admin.Handle("GET /dashboard", dashboard)
//
//	// Sub-groups inherit parent middleware
//	adminUsers := admin.Group("/users")
//	adminUsers.Handle("GET /{id}", adminGetUser)
func (r *Router) Group(prefix string, middleware ...MiddlewareFunc) *Group {
	return &Group{
		router:     r,
		prefix:     ensureLeadingSlash(prefix),
		middleware: middleware,
	}
}

// Handle registers a handler for the given pattern within the group.
// The group's prefix is automatically prepended to the pattern.
//
// Example:
//
//	users := r.Group("/api/users")
//	users.Handle("GET /{id}", getUser)
//	users.Handle("POST /", createUser)
//	users.Handle("DELETE /{id}", deleteUser)
//
//	// Equivalent to:
//	r.Handle("GET /api/users/{id}", getUser)
//	r.Handle("POST /api/users/", createUser)
//	r.Handle("DELETE /api/users/{id}", deleteUser)
func (g *Group) Handle(pattern string, handler func(*Context)) {
	method, path := splitMethodPath(pattern)
	fullPath := g.prefix + path
	fullPattern := method + " " + cleanPath(fullPath)
	g.router.handleWithMiddleware(fullPattern, handler, g.middleware)
}

// HandleWith registers a handler with additional per-route middleware within the group.
// The per-route middleware executes after group middleware but before the handler.
//
// Example:
//
//	users := r.Group("/api/users", authMiddleware)
//	users.HandleWith("DELETE /{id}", deleteUser, auditLog)
//	// Middleware order: authMiddleware -> auditLog -> deleteUser
func (g *Group) HandleWith(pattern string, handler func(*Context), middleware ...MiddlewareFunc) {
	method, path := splitMethodPath(pattern)
	fullPath := g.prefix + path
	fullPattern := method + " " + cleanPath(fullPath)

	// Combine group middleware with per-route middleware
	combined := combineMiddleware(g.middleware, middleware)

	g.router.handleWithMiddleware(fullPattern, handler, combined)
}

// HandleRaw registers a standard http.Handler for the given pattern within the group.
func (g *Group) HandleRaw(pattern string, handler http.Handler) {
	method, path := splitMethodPath(pattern)
	fullPath := g.prefix + path
	fullPattern := method + " " + cleanPath(fullPath)
	g.router.mux.Handle(fullPattern, handler)
}

// cleanPath removes duplicate slashes from combined path strings.
// This is needed when concatenating group prefix + route path.
// Handles multiple consecutive slashes (e.g., "///" → "//").
// Examples: "/api" + "/users" → "/api/users", "/" + "/users" → "/users"
//
// The cleanPath function is called at route registration time (setup), not per-request.
func cleanPath(path string) string {
	if path == "/" {
		return "/"
	}
	// Fast path: check if there are any consecutive slashes
	hasDoubleSlash := false
	for i := 1; i < len(path); i++ {
		if path[i] == '/' && path[i-1] == '/' {
			hasDoubleSlash = true
			break
		}
	}

	if !hasDoubleSlash {
		return path
	}

	// Slow path: replace "//" with "/" once (handles exactly one level of duplication)
	return strings.ReplaceAll(path, "//", "/")
}

// splitMethodPath splits a Go 1.22+ pattern into method and path components.
// Format: "METHOD /path/{param}" → ("METHOD", "/path/{param}")
// If pattern contains no space, returns ("", pattern) (assumes it's just a path).
func splitMethodPath(pattern string) (method, path string) {
	before, after, ok := strings.Cut(pattern, " ")
	if !ok {
		return "", pattern
	}
	return before, after
}

// Use appends middleware to this group. The middleware only applies to routes
// registered through this specific group (not the parent router).
//
// Example:
//
//	users := r.Group("/api/users")
//	users.Use(authMiddleware, loggingMiddleware)
//	users.Handle("GET /{id}", getUser)
func (g *Group) Use(m ...MiddlewareFunc) {
	g.middleware = append(g.middleware, m...)
}

// SubGroup creates a nested group with an additional prefix.
// The new group's prefix combines the parent's prefix with the new prefix.
// Middleware from the parent group is inherited and combined with any new middleware.
//
// Example:
//
//	users := r.Group("/api/users")
//	users.Use(authMiddleware)
//
//	posts := users.SubGroup("/posts")
//	posts.Handle("GET /{id}", getPost)
//	// Registers: GET /api/users/posts/{id}
//	// Middleware: authMiddleware (inherited from parent)
func (g *Group) SubGroup(prefix string, middleware ...MiddlewareFunc) *Group {
	fullPrefix := g.prefix + ensureLeadingSlash(prefix)

	// Combine parent middleware with new middleware
	combined := combineMiddleware(g.middleware, middleware)

	return &Group{
		router:     g.router,
		prefix:     fullPrefix,
		middleware: combined,
	}
}

// Prefix returns the group's current prefix.
func (g *Group) Prefix() string {
	return g.prefix
}

// handleWithMiddleware wraps a handler with middleware and registers it on the mux.
func (r *Router) handleWithMiddleware(pattern string, handler func(*Context), middleware []MiddlewareFunc) {
	if len(middleware) == 0 {
		r.mux.Handle(pattern, &zenHandler{fn: handler})
		return
	}

	// Build middleware chain from innermost to outermost
	h := http.Handler(&zenHandler{fn: handler})
	for i := len(middleware) - 1; i >= 0; i-- {
		h = &middlewareHandler{m: middleware[i], next: h}
	}

	// Wrap with context creation and release
	r.mux.Handle(pattern, &contextAwareHandler{chain: h})
}
