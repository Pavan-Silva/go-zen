package zen

import (
	"maps"
	"net/http"
	"net/url"
	"sync"
)

// zenCtxKey is used for storing Ctx inside standard request context instances.
type zenCtxKey struct{}

// HeaderXRequestID is the canonical header name for request IDs.
// Set on responses by the Request ID middleware.
const HeaderXRequestID = "X-Request-ID"

// HandlerFunc is the universal type for both route handlers and middleware.
type HandlerFunc func(*Ctx)

// SkipFunc decides whether a request should bypass middleware (auth, body limit, etc).
// It receives the current request and returns true for routes to skip.
type SkipFunc func(*http.Request) bool

// Ctx is the per-request context container for zen handlers and middleware components.
// Valid ONLY for the explicit lifetime of the request—never preserve references beyond handler returns.
type Ctx struct {
	Response     http.ResponseWriter
	Request      *http.Request
	store        map[string]any
	mu           sync.RWMutex
	next         HandlerFunc
	ps           params
	skippedNodes []skippedNode
	engine       *Engine
	queryCache   url.Values
}

// reset reinitializes Ctx for the given request and response.
func (c *Ctx) reset(w http.ResponseWriter, r *http.Request, e *Engine) {
	c.Response = w
	c.Request = r
	c.engine = e
	c.next = nil
	c.ps = c.ps[:0]
	c.skippedNodes = c.skippedNodes[:0]
	c.queryCache = nil
	c.mu.Lock()
	if c.store != nil {
		clear(c.store)
	}
	c.mu.Unlock()
}

// Next calls the next handler in the chain.
func (c *Ctx) Next() {
	if c.next != nil {
		c.next(c)
	}
}

// Set stores a value in the context. The internal map is allocated on first use.
func (c *Ctx) Set(key string, val any) {
	c.mu.Lock()
	if c.store == nil {
		c.store = make(map[string]any, 4)
	}
	c.store[key] = val
	c.mu.Unlock()
}

// Get retrieves a value from the context store.
func (c *Ctx) Get(key string) (any, bool) {
	c.mu.RLock()
	if c.store == nil {
		c.mu.RUnlock()
		return nil, false
	}
	val, ok := c.store[key]
	c.mu.RUnlock()
	return val, ok
}

// Keys returns a copy of all key-value entries in the store.
func (c *Ctx) Keys() map[string]any {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if c.store == nil {
		return nil
	}
	if len(c.store) == 0 {
		return nil
	}
	cp := make(map[string]any, len(c.store))
	maps.Copy(cp, c.store)
	return cp
}

// Copy returns a new Ctx with a deep copy of the store.
func (c *Ctx) Copy() *Ctx {
	cp := &Ctx{
		Response: c.Response,
		Request:  c.Request,
		engine:   c.engine,
	}
	if len(c.ps) > 0 {
		cp.ps = make(params, len(c.ps))
		copy(cp.ps, c.ps)
	}
	c.mu.RLock()
	if len(c.store) > 0 {
		cp.store = make(map[string]any, len(c.store))
		maps.Copy(cp.store, c.store)
	}
	c.mu.RUnlock()
	return cp
}

// FromRequest extracts a Ctx from an http.Request.
func FromRequest(r *http.Request) (*Ctx, bool) {
	c, ok := r.Context().Value(zenCtxKey{}).(*Ctx)
	return c, ok
}
