package zen

import (
	"context"
	"maps"
	"net/http"
	"sync"
)

// zenCtxKey is used as the key for storing Context in the request context.
// It's an unexported type to ensure no collisions with other packages.
type zenCtxKey struct{}

// contextPool caches allocated Context instances for reuse.
// This reduces GC pressure and improves performance under high load.
var contextPool = sync.Pool{
	New: func() any {
		return &Context{}
	},
}

// Context is the request context for zen handlers. It provides a simple
// key-value store using a map, matching Gin and Echo's approach.
//
// Context is valid only for the lifetime of the request.
// Do not store references to it beyond the handler's return.
type Context struct {
	// Response is the http.ResponseWriter for this request.
	Response http.ResponseWriter
	// Request is the *http.Request for this request.
	Request *http.Request
	// store holds key-value pairs for the request context.
	// Lazily initialized on first Set() call.
	store map[string]any
	// mu protects store from concurrent access.
	mu sync.RWMutex
}

// reset clears the Context for reuse.
func (c *Context) reset(w http.ResponseWriter, r *http.Request) {
	c.Response = w
	c.Request = r
	c.store = nil
	c.mu = sync.RWMutex{}
}

// Set stores a value in the context with the given key.
func (c *Context) Set(key string, val any) {
	c.mu.Lock()
	if c.store == nil {
		c.store = make(map[string]any)
	}
	c.store[key] = val
	c.mu.Unlock()
}

// Get retrieves a value from the context by key, returning the value and a boolean
// indicating whether the key exists. This distinguishes between a key that was
// explicitly set to nil and a key that was never set.
func (c *Context) Get(key string) (any, bool) {
	c.mu.RLock()
	if c.store == nil {
		c.mu.RUnlock()
		return nil, false
	}
	val, ok := c.store[key]
	c.mu.RUnlock()
	return val, ok
}

// Keys returns a copy of the context's key-value store.
// Useful for logging, tracing, and middleware propagation.
func (c *Context) Keys() map[string]any {
	c.mu.RLock()
	if c.store == nil {
		c.mu.RUnlock()
		return nil
	}
	cp := make(map[string]any, len(c.store))
	maps.Copy(cp, c.store)
	c.mu.RUnlock()
	return cp
}

// Copy returns a new Context with a deep copy of the store map.
// The copy is heap-allocated and NOT pooled — the caller owns it.
// Use this to pass context values to goroutines that outlive the request.
func (c *Context) Copy() *Context {
	cp := &Context{
		Response: c.Response,
		Request:  c.Request,
	}
	c.mu.RLock()
	if c.store != nil {
		cp.store = make(map[string]any, len(c.store))
		maps.Copy(cp.store, c.store)
	}
	c.mu.RUnlock()
	return cp
}

// newContext retrieves a Context from the pool, initializes it, and stores
// it in the request context for retrieval by handlers that need FromRequest.
func newContext(w http.ResponseWriter, r *http.Request) (*Context, *http.Request) {
	c := contextPool.Get().(*Context)
	c.reset(w, r)
	r = r.WithContext(context.WithValue(r.Context(), zenCtxKey{}, c))
	c.Request = r
	return c, r
}

// newContextNoRequest retrieves a Context from the pool and initializes it
// without storing it in the request context. This avoids one allocation for
// middleware-heavy request paths where handlers are invoked directly.
func newContextNoRequest(w http.ResponseWriter, r *http.Request) (*Context, *http.Request) {
	c := contextPool.Get().(*Context)
	c.reset(w, r)
	return c, r
}

// releaseContext returns the Context to the pool after resetting it.
// Must be called exactly once per newContext call. Calling twice
// will put the same pointer into the pool for two goroutines to
// retrieve, causing data races on the recycled Context.
func releaseContext(c *Context) {
	c.reset(nil, nil)
	contextPool.Put(c)
}

// FromContext retrieves the zen Context from a context.Context.
// Returns (nil, false) if no zen Context is present.
func FromContext(ctx context.Context) (*Context, bool) {
	c, ok := ctx.Value(zenCtxKey{}).(*Context)
	return c, ok
}

// FromRequest retrieves the zen Context from an *http.Request.
// Returns (nil, false) if no zen Context is present.
func FromRequest(r *http.Request) (*Context, bool) {
	return FromContext(r.Context())
}
