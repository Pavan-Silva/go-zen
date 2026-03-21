package zen

import (
	"net/http"
	"strings"
)

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
//	// Registers: GET /api/users/{id}
func (r *Router) Group(prefix string, middleware ...MiddlewareFunc) *Group {
	if prefix != "" && !strings.HasPrefix(prefix, "/") {
		prefix = "/" + prefix
	}
	return &Group{
		router:     r,
		prefix:     prefix,
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

// HandleRaw registers a standard http.Handler for the given pattern within the group.
func (g *Group) HandleRaw(pattern string, handler http.Handler) {
	method, path := splitMethodPath(pattern)
	fullPath := g.prefix + path
	fullPattern := method + " " + cleanPath(fullPath)
	g.router.mux.Handle(fullPattern, handler)
}

// cleanPath removes duplicate slashes from the path.
// e.g., "/api" + "/users" becomes "/api/users", and "/" + "/users" becomes "/users".
func cleanPath(path string) string {
	if path == "/" {
		return "/"
	}
	// Remove duplicate slashes
	for strings.Contains(path, "//") {
		path = strings.Replace(path, "//", "/", -1)
	}
	return path
}

// splitMethodPath splits a Go 1.22+ pattern like "GET /users/{id}" into
// method and path components. Returns ("GET", "/users/{id}").
func splitMethodPath(pattern string) (method, path string) {
	idx := strings.IndexByte(pattern, ' ')
	if idx == -1 {
		return "", pattern
	}
	return pattern[:idx], pattern[idx+1:]
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
	if prefix != "" && !strings.HasPrefix(prefix, "/") {
		prefix = "/" + prefix
	}
	fullPrefix := g.prefix + prefix

	// Combine parent middleware with new middleware
	combined := make([]MiddlewareFunc, 0, len(g.middleware)+len(middleware))
	combined = append(combined, g.middleware...)
	combined = append(combined, middleware...)

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

// contextAwareHandler creates and releases zen Context for routes with middleware.
type contextAwareHandler struct {
	chain http.Handler
}

func (h *contextAwareHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	c, r := newContext(w, r)
	h.chain.ServeHTTP(w, r)
	releaseContext(c)
}
