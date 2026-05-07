package zen

import (
	"context"
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
type Context struct {
	// Response is the http.ResponseWriter for this request.
	Response http.ResponseWriter
	// Request is the *http.Request for this request.
	Request *http.Request
	// store holds key-value pairs for the request context.
	// Lazily initialized on first Set() call.
	store map[string]any
	// mu protects store from concurrent access.
	// Lazily initialized on first Set() call to reduce Context size.
	mu *sync.RWMutex
}

// reset clears the Context for reuse.
func (c *Context) reset(w http.ResponseWriter, r *http.Request) {
	c.Response = w
	c.Request = r
	c.store = nil
}

// Set stores a value in the context with the given key.
//
// Example:
//
//	c.Set("user_id", 42)
//	c.Set("request_id", uuid.New().String())
func (c *Context) Set(key string, val any) {
	if c.mu == nil {
		c.mu = &sync.RWMutex{}
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.store == nil {
		c.store = make(map[string]any)
	}
	c.store[key] = val
}

// Get retrieves a value from the context by key, returning the value and a boolean
// indicating whether the key exists. This distinguishes between a key that was
// explicitly set to nil and a key that was never set.
//
// Example:
//
//	userID, ok := c.Get("user_id")
//	if !ok {
//	    c.Error(http.StatusUnauthorized, "unauthorized")
//	    return
//	}
func (c *Context) Get(key string) (any, bool) {
	if c.mu == nil {
		return nil, false
	}
	c.mu.RLock()
	defer c.mu.RUnlock()
	if c.store == nil {
		return nil, false
	}
	val, ok := c.store[key]
	return val, ok
}

// SetResponse sets the http.ResponseWriter for this context.
func (c *Context) SetResponse(w http.ResponseWriter) {
	c.Response = w
}

// newContext retrieves a Context from the pool, initializes it, and stores
// it in the request context for retrieval by middleware and handlers.
func newContext(w http.ResponseWriter, r *http.Request) (*Context, *http.Request) {
	c := contextPool.Get().(*Context)
	r = r.WithContext(context.WithValue(r.Context(), zenCtxKey{}, c))
	c.reset(w, r)
	return c, r
}

// releaseContext returns the Context to the pool after resetting it.
func releaseContext(c *Context) {
	c.reset(nil, nil)
	contextPool.Put(c)
}

// FromRequest retrieves the Context from an *http.Request if it exists.
func FromRequest(r *http.Request) *Context {
	if c := r.Context().Value(zenCtxKey{}); c != nil {
		return c.(*Context)
	}
	return nil
}
