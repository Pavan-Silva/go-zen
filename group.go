package zen

import (
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

// Handle parses a "METHOD /path" pattern string and registers the handler.
// If no method is present, GET is assumed.
func (g *RouterGroup) Handle(pattern string, handler HandlerFunc) {
	method, path := splitMethodPath(pattern)
	g.handle(method, path, handler)
}

// HandleWith registers a handler with per-route middleware.
// The per-route middleware runs after group middleware but before the handler.
func (g *RouterGroup) HandleWith(pattern string, handler HandlerFunc, middleware ...HandlerFunc) {
	method, path := splitMethodPath(pattern)
	g.registerRoute(method, g.prefix+path, handler, middleware)
}

// HandleRaw registers a raw http.Handler directly on the underlying ServeMux,
// bypassing the zen middleware chain for native performance.
func (g *RouterGroup) HandleRaw(pattern string, handler http.Handler) {
	method, path := splitMethodPath(pattern)
	g.engine.Mux.Handle(method+" "+g.prefix+path, handler)
}

// Prefix returns the full path prefix of this group.
func (g *RouterGroup) Prefix() string { return g.prefix }

// Core single point of entry for adding routes with group middleware.
func (g *RouterGroup) handle(method, pattern string, handler HandlerFunc) {
	g.registerRoute(method, g.prefix+pattern, handler, nil)
}

// Unified route compiler and executor block.
// Compiles the entire handler and middleware chain backwards at startup into a
// single, nested closure link. This completely eliminates slice tracking or
// index mutations within the hot request path.
func (g *RouterGroup) registerRoute(method, fullPath string, handler HandlerFunc, routeMiddleware []HandlerFunc) {
	fullPattern := method + " " + fullPath

	totalLen := len(g.middleware) + len(routeMiddleware) + 1
	chain := make([]HandlerFunc, totalLen)
	copy(chain, g.middleware)
	copy(chain[len(g.middleware):], routeMiddleware)
	chain[totalLen-1] = handler

	var finalHandler HandlerFunc = chain[len(chain)-1]

	for i := len(chain) - 2; i >= 0; i-- {
		nextHandler := finalHandler
		currentMiddleware := chain[i]

		finalHandler = func(c *Ctx) {
			c.next = nextHandler
			currentMiddleware(c)
		}
	}

	g.engine.Mux.HandleFunc(fullPattern, func(w http.ResponseWriter, r *http.Request) {
		c := g.engine.pool.Get().(*Ctx)
		c.reset(w, r)

		finalHandler(c)

		g.engine.pool.Put(c)
	})
}

// ensureLeadingSlash sanitizes strings to guarantee an HTTP path compliance scheme.
func ensureLeadingSlash(prefix string) string {
	if prefix == "" {
		return "/"
	}
	if prefix[0] != '/' {
		return "/" + prefix
	}
	return prefix
}

// splitMethodPath cuts method headers away from raw registration keys efficiently.
func splitMethodPath(pattern string) (method, path string) {
	pattern = strings.TrimSpace(pattern)
	before, after, ok := strings.Cut(pattern, " ")

	if !ok {
		return "GET", ensureLeadingSlash(pattern)
	}

	method = strings.ToUpper(before)
	path = ensureLeadingSlash(strings.TrimSpace(after))
	return method, path
}

// Convenience methods for common HTTP methods
func (g *RouterGroup) GET(p string, h HandlerFunc)    { g.handle("GET", p, h) }
func (g *RouterGroup) POST(p string, h HandlerFunc)   { g.handle("POST", p, h) }
func (g *RouterGroup) PUT(p string, h HandlerFunc)    { g.handle("PUT", p, h) }
func (g *RouterGroup) DELETE(p string, h HandlerFunc) { g.handle("DELETE", p, h) }
func (g *RouterGroup) PATCH(p string, h HandlerFunc)  { g.handle("PATCH", p, h) }
func (g *RouterGroup) HEAD(p string, h HandlerFunc)   { g.handle("HEAD", p, h) }
