package zen

import (
	"context"
	"net/http"
	"net/url"
	"sync"
)

// contextPool reuses Context objects across requests to avoid per-request heap allocations.
var contextPool = sync.Pool{
	New: func() any { return &Context{} },
}

// zenCtxKey is the key used to store *Context in the request context
// so middlewares can retrieve it without an extra allocation per middleware.
type zenCtxKey struct{}

type Context struct {
	Response   http.ResponseWriter
	Request    *http.Request
	queryCache url.Values
	values     map[string]any // middleware/handler shared storage
}

// reset prepares the context for a new request, or zeros it out when called
// with nil values before returning to the pool. Accepting nil here means the
// pool-return path in adaptHandler can call c.reset(nil, nil) instead
// of manually clearing each field — ensuring no field is ever forgotten when
// Context gains new fields in the future.
func (c *Context) reset(w http.ResponseWriter, r *http.Request) {
	c.Response = w
	c.Request = r
	c.queryCache = nil
	c.values = nil // lazy-initialised on first Set
}

// Set stores a value under key, shared across middlewares and the handler
// for the lifetime of the request. The map is allocated lazily on first use.
func (c *Context) Set(key string, val any) {
	if c.values == nil {
		c.values = make(map[string]any)
	}
	c.values[key] = val
}

// Get retrieves a value previously stored with Set.
// Returns nil if the key is not present.
func (c *Context) Get(key string) any {
	return c.values[key]
}

// adaptHandler turns a zen-style handler into a standard http.Handler.
// It is used at route registration time and reuses Context instances via contextPool.
func adaptHandler(handler func(*Context)) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c := contextPool.Get().(*Context)
		c.reset(w, r)

		// Attach *Context to the request context once so any middleware in
		// the chain can call zen.FromRequest(r) to reach it.
		r = r.WithContext(context.WithValue(r.Context(), zenCtxKey{}, c))
		c.Request = r

		// Nil references before returning to pool, defer to survive panics
		defer func() {
			c.reset(nil, nil)
			contextPool.Put(c)
		}()

		handler(c)
	})
}

// FromRequest retrieves the *Context from a standard http.Request.
// Returns nil if the request did not go through a zen handler.
func FromRequest(r *http.Request) *Context {
	c, _ := r.Context().Value(zenCtxKey{}).(*Context)
	return c
}
