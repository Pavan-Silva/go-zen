package zen

import (
	"context"
	"net/http"
	"sync"
)

// contextPool reuses Context allocations across requests to reduce GC pressure.
// This is a key performance optimization for high-throughput servers.
var contextPool = sync.Pool{
	New: func() any { return newContextPooled() },
}

// newContextPooled creates a new Context with pre-allocated storage.
// The overflow map is allocated upfront to avoid allocations on first Set().
func newContextPooled() *Context {
	return &Context{
		overflow: make(map[string]any, 4),
	}
}

// zenCtxKey is used as the key for storing Context in the request context.
// It's an unexported type to ensure no collisions with other packages.
type zenCtxKey struct{}

// kv represents a key-value pair for the context store.
type kv struct {
	k string
	v any
}

// inlineSlots is the number of key-value pairs stored inline to avoid allocations.
// This is tuned for typical small request contexts (middleware, request IDs, etc).
const inlineSlots = 8

// Context is the request context for zen handlers. It provides a lightweight
// alternative to using context.Context directly, offering faster Set/Get operations
// through inline slots before falling back to a map for overflow.
//
// The design balances performance (inline storage for common cases) with
// flexibility (overflow map for advanced use cases).
type Context struct {
	// Response is the http.ResponseWriter for this request.
	Response http.ResponseWriter
	// Request is the *http.Request for this request.
	Request *http.Request
	// queryCache caches parsed query parameters (lazy initialization).
	queryCache map[string]string
	// store holds the first inlineSlots key-value pairs inline to avoid allocations.
	store [inlineSlots]kv
	// storeLen tracks how many slots are used in store.
	storeLen int
	// overflow holds additional key-value pairs after inlineSlots are exhausted.
	overflow map[string]any
}

// reset clears the Context for reuse from the pool. This is called before
// returning to the pool and ensures no data leaks between requests.
func (c *Context) reset(w http.ResponseWriter, r *http.Request) {
	c.Response = w
	c.Request = r
	c.queryCache = nil
	// Clear inline slots to avoid retaining references across pooled requests.
	for i := 0; i < c.storeLen; i++ {
		c.store[i] = kv{}
	}
	c.storeLen = 0
	// Clear overflow map by deleting all keys (keeps capacity for next use).
	for k := range c.overflow {
		delete(c.overflow, k)
	}
}

// Set stores a value in the context with the given key. Values are stored
// inline (fast path) for the first 8 keys, then overflow to a map.
// This design provides O(1) performance for common use cases without map overhead.
//
// Example:
//
//	c.Set("user_id", 42)
//	c.Set("request_id", uuid.New().String())
func (c *Context) Set(key string, val any) {
	// Fast path: check if key already exists in inline slots.
	for i := 0; i < c.storeLen; i++ {
		if c.store[i].k == key {
			c.store[i].v = val
			return
		}
	}
	// Check if we have room in inline slots.
	if c.storeLen < len(c.store) {
		c.store[c.storeLen].k = key
		c.store[c.storeLen].v = val
		c.storeLen++
		return
	}
	// Overflow to map for additional keys.
	c.overflow[key] = val
}

// Get retrieves a value from the context by key, returning nil if not found.
// Values are looked up inline first (O(n) where n ≤ 8), then in the overflow map (O(1)).
//
// Example:
//
//	userID := c.Get("user_id")
//	if userID == nil {
//	    c.Error(http.StatusUnauthorized, "unauthorized")
//	    return
//	}
func (c *Context) Get(key string) any {
	// Fast path: search inline slots (typically very small).
	for i := 0; i < c.storeLen; i++ {
		if c.store[i].k == key {
			return c.store[i].v
		}
	}
	// Check overflow map without initializing if empty.
	v, _ := c.overflow[key]
	return v
}

// newContext retrieves a Context from the pool, initializes it, and stores
// it in the request context for retrieval by middleware and handlers.
// This function allocates a new Context from the pool exactly once per request.
func newContext(w http.ResponseWriter, r *http.Request) (*Context, *http.Request) {
	c := contextPool.Get().(*Context)
	r = r.WithContext(context.WithValue(r.Context(), zenCtxKey{}, c))
	c.reset(w, r)
	return c, r
}

// releaseContext returns a Context to the pool for reuse after clearing all data.
// This must be called for every Context obtained from newContext to prevent leaks.
func releaseContext(c *Context) {
	c.reset(nil, nil)
	contextPool.Put(c)
}

// FromRequest retrieves the zenContext from an *http.Request if it exists.
// Returns nil if no Context has been attached (should only happen if called outside
// the zen request lifecycle or through a raw http.Handler).
//
// This is the standard way to access the Context in middleware and handlers.
func FromRequest(r *http.Request) *Context {
	if c := r.Context().Value(zenCtxKey{}); c != nil {
		return c.(*Context)
	}
	return nil
}
