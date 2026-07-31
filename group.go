package zen

import (
	"context"
	"net/http"
	"strings"
)

// RouterGroup groups routes under a common prefix with optional shared middleware.
// An Engine embeds a root RouterGroup; Group creates child groups.
type RouterGroup struct {
	prefix     string
	engine     *Engine
	middleware []HandlerFunc
}

// Use appends middleware to the group. Middleware runs for every route registered on this group.
func (g *RouterGroup) Use(middleware ...HandlerFunc) {
	g.middleware = append(g.middleware, middleware...)
	if g.engine != nil && g.prefix == "" {
		// Root-group middleware also feeds the 404/405/TSR chains.
		g.engine.rebuildNoRouteChains()
	}
}

// Group creates a child group with an additional path prefix and optional middleware.
// The child group inherits all parent middleware and appends the new middleware after it.
// Uses a single upfront allocation to prevent double-allocation slice resizes.
func (g *RouterGroup) Group(prefix string, middleware ...HandlerFunc) *RouterGroup {
	totalLen := len(g.middleware) + len(middleware)
	combined := make([]HandlerFunc, totalLen)
	copy(combined, g.middleware)
	copy(combined[len(g.middleware):], middleware)

	return &RouterGroup{
		prefix:     g.prefix + ensureLeadingSlash(prefix),
		engine:     g.engine,
		middleware: combined,
	}
}

// HandleRaw registers a raw http.Handler on the router using the zen handler chain.
// The pattern uses Go 1.22+ ServeMux syntax ({param}, {path...}) which is
// automatically converted to the router's native syntax (:param, *path).
// The group prefix is prepended to the path automatically.
//
// The raw handler can recover the current Ctx from the request with
// FromRequest (and must not retain it beyond the request lifetime).
func (g *RouterGroup) HandleRaw(pattern string, handler http.Handler) {
	method, path := convertServeMuxPattern(pattern)
	g.registerRoute(method, g.prefix+path, []HandlerFunc{func(c *Ctx) {
		req := c.Request.WithContext(context.WithValue(c.Request.Context(), zenCtxKey{}, c))
		handler.ServeHTTP(c.Response, req)
	}})
}

// Prefix returns the full path prefix of this group.
func (g *RouterGroup) Prefix() string { return g.prefix }

// GET registers a handler for the GET method.
// The final element in handlers is the main handler; preceding elements are middleware.
func (g *RouterGroup) GET(path string, handlers ...HandlerFunc) {
	g.registerRoute("GET", g.prefix+path, handlers)
}

// POST registers a handler for the POST method.
func (g *RouterGroup) POST(path string, handlers ...HandlerFunc) {
	g.registerRoute("POST", g.prefix+path, handlers)
}

// PUT registers a handler for the PUT method.
func (g *RouterGroup) PUT(path string, handlers ...HandlerFunc) {
	g.registerRoute("PUT", g.prefix+path, handlers)
}

// DELETE registers a handler for the DELETE method.
func (g *RouterGroup) DELETE(path string, handlers ...HandlerFunc) {
	g.registerRoute("DELETE", g.prefix+path, handlers)
}

// PATCH registers a handler for the PATCH method.
func (g *RouterGroup) PATCH(path string, handlers ...HandlerFunc) {
	g.registerRoute("PATCH", g.prefix+path, handlers)
}

// HEAD registers a handler for the HEAD method.
func (g *RouterGroup) HEAD(path string, handlers ...HandlerFunc) {
	g.registerRoute("HEAD", g.prefix+path, handlers)
}

// OPTIONS registers a handler for the OPTIONS method.
func (g *RouterGroup) OPTIONS(path string, handlers ...HandlerFunc) {
	g.registerRoute("OPTIONS", g.prefix+path, handlers)
}

// registerRoute registers a route with the given method, path, and handlers.
func (g *RouterGroup) registerRoute(method, fullPath string, handlers []HandlerFunc) {
	if method == "" {
		panic("zen: route registered without an HTTP method (use the \"METHOD /path\" pattern, e.g. \"GET /hello\")")
	}
	if len(handlers) == 0 {
		panic("zen: route registered with zero handlers")
	}

	// The last function is always the final business logic handler
	handler := handlers[len(handlers)-1]
	routeMiddleware := handlers[:len(handlers)-1]

	// Build the full array of handlers for this route at startup
	totalLen := len(g.middleware) + len(routeMiddleware) + 1
	chain := make([]HandlerFunc, totalLen)
	copy(chain, g.middleware)
	copy(chain[len(g.middleware):], routeMiddleware)
	chain[totalLen-1] = handler

	g.engine.router.add(method, convertPath(fullPath), []HandlerFunc{compileChain(chain)})
}

// compileChain builds a nested closure chain where each middleware forwards to
// the next via the Ctx.next field. The final element runs last.
func compileChain(chain []HandlerFunc) HandlerFunc {
	var finalHandler = chain[len(chain)-1]

	// Compile the chain backwards into a nested closure chain
	for i := len(chain) - 2; i >= 0; i-- {
		nextHandler := finalHandler
		currentMiddleware := chain[i]

		finalHandler = func(c *Ctx) {
			c.next = nextHandler
			currentMiddleware(c)
		}
	}

	return finalHandler
}

// ensureLeadingSlash adds a leading slash (if absent) and strips a trailing slash.
func ensureLeadingSlash(prefix string) string {
	if prefix == "" {
		return "/"
	}
	prefix = strings.TrimSuffix(prefix, "/")
	if prefix[0] != '/' {
		return "/" + prefix
	}
	return prefix
}
