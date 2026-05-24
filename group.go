package zen

import (
	"net/http"
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

// HandleRaw registers a raw http.Handler directly on the underlying ServeMux,
// bypassing the zen middleware chain for native performance.
func (g *RouterGroup) HandleRaw(pattern string, handler http.Handler) {
	g.engine.Mux.Handle(pattern, handler)
}

// Prefix returns the full path prefix of this group.
func (g *RouterGroup) Prefix() string { return g.prefix }

// Convenience methods for common HTTP methods.
// Uses a fully variadic handlers list matching Gin's signature design.
// The final element in the variadic slice is treated as the main endpoint handler.
func (g *RouterGroup) GET(path string, handlers ...HandlerFunc) {
	g.registerRoute("GET", g.prefix+path, handlers)
}

func (g *RouterGroup) POST(path string, handlers ...HandlerFunc) {
	g.registerRoute("POST", g.prefix+path, handlers)
}

func (g *RouterGroup) PUT(path string, handlers ...HandlerFunc) {
	g.registerRoute("PUT", g.prefix+path, handlers)
}

func (g *RouterGroup) DELETE(path string, handlers ...HandlerFunc) {
	g.registerRoute("DELETE", g.prefix+path, handlers)
}

func (g *RouterGroup) PATCH(path string, handlers ...HandlerFunc) {
	g.registerRoute("PATCH", g.prefix+path, handlers)
}

func (g *RouterGroup) HEAD(path string, handlers ...HandlerFunc) {
	g.registerRoute("HEAD", g.prefix+path, handlers)
}

func (g *RouterGroup) OPTIONS(path string, handlers ...HandlerFunc) {
	g.registerRoute("OPTIONS", g.prefix+path, handlers)
}

// Unified route compiler and executor block.
// Extracts the final handler from the variadic slice and appends route-specific middleware.
func (g *RouterGroup) registerRoute(method, fullPath string, handlers []HandlerFunc) {
	if len(handlers) == 0 {
		panic("zen: route registered with zero handlers")
	}

	fullPattern := method + " " + fullPath

	// The last function is always the final business logic handler
	handler := handlers[len(handlers)-1]
	routeMiddleware := handlers[:len(handlers)-1]

	// Build the full array of handlers for this route at startup
	totalLen := len(g.middleware) + len(routeMiddleware) + 1
	chain := make([]HandlerFunc, totalLen)
	copy(chain, g.middleware)
	copy(chain[len(g.middleware):], routeMiddleware)
	chain[totalLen-1] = handler

	var finalHandler HandlerFunc = chain[len(chain)-1]

	// Compile the chain backwards into a nested closure chain
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
