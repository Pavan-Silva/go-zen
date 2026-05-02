package zen

import (
	"context"
	"net/http"
	"sync"
)

// zenCtxKey is used as the key for storing Context in the request context.
// It's an unexported type to ensure no collisions with other packages.
type zenCtxKey struct{}

// Context is the request context for zen handlers. It provides a simple
// key-value store using a map, matching Gin and Echo's approach.
//
// A RWMutex protects the store for thread safety, similar to Echo.
// While each request typically has its own Context, the pool pattern means
// the same context could theoretically be accessed concurrently.
type Context struct {
	// Response is the http.ResponseWriter for this request.
	Response http.ResponseWriter
	// Request is the *http.Request for this request.
	Request *http.Request
	// store holds key-value pairs for the request context.
	store map[string]any
	mu    sync.RWMutex
}

// reset clears the Context for reuse.
func (c *Context) reset(w http.ResponseWriter, r *http.Request) {
	c.Response = w
	c.Request = r
	c.mu.Lock()
	c.store = nil
	c.mu.Unlock()
}

// Set stores a value in the context with the given key.
//
// Example:
//
//	c.Set("user_id", 42)
//	c.Set("request_id", uuid.New().String())
func (c *Context) Set(key string, val any) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.store == nil {
		c.store = make(map[string]any)
	}
	c.store[key] = val
}

// Get retrieves a value from the context by key, returning nil if not found.
//
// Example:
//
//	userID := c.Get("user_id")
//	if userID == nil {
//	    c.Error(http.StatusUnauthorized, "unauthorized")
//	    return
//	}
func (c *Context) Get(key string) any {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.store[key]
}

// newContext creates a fresh Context, initializes it, and stores
// it in the request context for retrieval by middleware and handlers.
func newContext(w http.ResponseWriter, r *http.Request) (*Context, *http.Request) {
	c := &Context{}
	r = r.WithContext(context.WithValue(r.Context(), zenCtxKey{}, c))
	c.reset(w, r)
	return c, r
}

// releaseContext is a no-op since we don't pool contexts.
func releaseContext(c *Context) {
	c.reset(nil, nil)
}

// FromRequest retrieves the Context from an *http.Request if it exists.
func FromRequest(r *http.Request) *Context {
	if c := r.Context().Value(zenCtxKey{}); c != nil {
		return c.(*Context)
	}
	return nil
}
