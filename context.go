package zen

import (
	"context"
	"maps"
	"net/http"
	"sync"
)

// zenCtxKey is used for storing Ctx inside standard request context instances.
type zenCtxKey struct{}

// Ctx is the per-request context container for zen handlers and middleware components.
// Valid ONLY for the explicit lifetime of the request—never preserve references beyond handler returns.
type Ctx struct {
	Response http.ResponseWriter
	Request  *http.Request
	store    map[string]any
	mu       sync.RWMutex
	next     HandlerFunc
}

// reset reinitializes the Ctx properties efficiently without sacrificing memory allocations.
// Leverages clear() to keep backing map bucket memories intact, bypassing reallocations.
func (c *Ctx) reset(w http.ResponseWriter, r *http.Request) {
	c.Response = w
	c.Request = r
	c.next = nil

	if c.store != nil {
		clear(c.store)
	}
}

// Next advances execution to the sequential middleware link inside the pre-compiled chain.
func (c *Ctx) Next() {
	if c.next != nil {
		c.next(c)
	}
}

// Set stores a key-value value inside the metadata context.
// Lazily allocates the internal map space on first use to prevent useless allocations on clean routes.
func (c *Ctx) Set(key string, val any) {
	c.mu.Lock()
	if c.store == nil {
		c.store = make(map[string]any, 4) // Tiny size optimization window limits footprint
	}
	c.store[key] = val
	c.mu.Unlock()
}

// Get retrieves a key metadata element from the internal store.
// Fast Path: Directly returns zero values if the backing map was never initialized,
// entirely sidestepping RLock contention overhead on hot routes.
func (c *Ctx) Get(key string) (any, bool) {
	if c.store == nil {
		return nil, false
	}
	c.mu.RLock()
	val, ok := c.store[key]
	c.mu.RUnlock()
	return val, ok
}

// Keys returns a thread-safe deep copy of all key-value entries inside the metadata store.
func (c *Ctx) Keys() map[string]any {
	if len(c.store) == 0 {
		return nil
	}
	c.mu.RLock()
	cp := make(map[string]any, len(c.store))
	maps.Copy(cp, c.store)
	c.mu.RUnlock()
	return cp
}

// Copy returns a new separate Ctx with a deep copy of the active store map.
// The clone is uniquely heap-allocated and NOT pooled—the caller takes absolute ownership.
func (c *Ctx) Copy() *Ctx {
	cp := &Ctx{
		Response: c.Response,
		Request:  c.Request,
	}
	if len(c.store) > 0 {
		c.mu.RLock()
		cp.store = make(map[string]any, len(c.store))
		maps.Copy(cp.store, c.store)
		c.mu.RUnlock()
	}
	return cp
}

// FromContext extracts a pooled zen Ctx container out of a standard context.Context.
func FromContext(ctx context.Context) (*Ctx, bool) {
	c, ok := ctx.Value(zenCtxKey{}).(*Ctx)
	return c, ok
}

// FromRequest extracts a pooled zen Ctx container out of an incoming http.Request reference.
func FromRequest(r *http.Request) (*Ctx, bool) {
	return FromContext(r.Context())
}
