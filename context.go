package zen

import (
	"context"
	"maps"
	"net/http"
	"sync"
)

// zenCtxKey is used for storing Ctx in request context.
type zenCtxKey struct{}

// Ctx is the per-request context for zen handlers and middleware.
// Valid only for the lifetime of the request — do not store beyond handler return.
type Ctx struct {
	Response http.ResponseWriter
	Request  *http.Request
	store    map[string]any
	mu       sync.RWMutex
	next     HandlerFunc
}

// reset reinitializes the Ctx properties efficiently without sacrificing memory allocations.
func (c *Ctx) reset(w http.ResponseWriter, r *http.Request) {
	c.Response = w
	c.Request = r

	// Zero-Allocation Clear: Retains map memory space while removing keys
	if c.store != nil {
		clear(c.store)
	}
}

// Next advances execution to the next handler in the chain.
// Handlers can call Next to invoke the subsequent handler in the middleware
// chain. If a handler returns without calling Next, the chain stops
// (short-circuit). Handlers may also run code after the call to Next
// for post-request logic.
func (c *Ctx) Next() {
	if c.next != nil {
		c.next(c)
	}
}

// Set stores a value in the context with the given key.
func (c *Ctx) Set(key string, val any) {
	c.mu.Lock()
	if c.store == nil {
		c.store = make(map[string]any, 4) // Initialize with small capacity to avoid resizes
	}
	c.store[key] = val
	c.mu.Unlock()
}

// Get retrieves a value from the context by key.
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

// Keys returns a copy of the context's key-value store.
func (c *Ctx) Keys() map[string]any {
	c.mu.RLock()
	if len(c.store) == 0 {
		c.mu.RUnlock()
		return nil
	}
	cp := make(map[string]any, len(c.store))
	maps.Copy(cp, c.store)
	c.mu.RUnlock()
	return cp
}

// Copy returns a new Ctx with a deep copy of the store map.
// The copy is heap-allocated and NOT pooled — the caller owns it.
func (c *Ctx) Copy() *Ctx {
	cp := &Ctx{
		Response: c.Response,
		Request:  c.Request,
	}
	c.mu.RLock()
	if len(c.store) > 0 {
		cp.store = make(map[string]any, len(c.store))
		maps.Copy(cp.store, c.store)
	}
	c.mu.RUnlock()
	return cp
}

// FromContext retrieves the zen Ctx from a context.Context.
func FromContext(ctx context.Context) (*Ctx, bool) {
	c, ok := ctx.Value(zenCtxKey{}).(*Ctx)
	return c, ok
}

// FromRequest retrieves the zen Ctx from an *http.Request.
func FromRequest(r *http.Request) (*Ctx, bool) {
	return FromContext(r.Context())
}
