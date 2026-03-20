package zen

import (
	"context"
	"net/http"
	"sync"
)

var contextPool = sync.Pool{
	New: func() any { return newContextPooled() },
}

func newContextPooled() *Context {
	return &Context{
		overflow: strMap{m: make(map[string]any, 4)},
	}
}

type zenCtxKey struct{}

type kv struct {
	k string
	v any
}

type strMap struct {
	m map[string]any
}

func (s *strMap) Get(k string) (any, bool) {
	v, ok := s.m[k]
	return v, ok
}

func (s *strMap) Set(k string, v any) {
	s.m[k] = v
}

func (s *strMap) Len() int {
	return len(s.m)
}

func (s *strMap) Clear() {
	for k := range s.m {
		delete(s.m, k)
	}
}

// Context is the request context passed to every zen handler and middleware.
type Context struct {
	Response   http.ResponseWriter
	Request    *http.Request
	queryCache map[string]string
	store      [8]kv
	storeLen   int
	overflow   strMap
}

// reset clears all fields to prepare the Context for reuse.
// It is called by [newContext] before handling each request and by
// [releaseContext] when returning the Context to the pool.
// Resetting storeLen to 0 means Set/Get will overwrite slots rather than
// clear them — this is safe because Set always writes key and value before
// incrementing storeLen.
func (c *Context) reset(w http.ResponseWriter, r *http.Request) {
	c.Response = w
	c.Request = r
	c.queryCache = nil
	c.storeLen = 0
	c.overflow.Clear()
}

// Set stores a value under key for the lifetime of the current request.
// Values set in middleware are available in handlers via [Context.Get].
//
// The first eight distinct keys are stored inline with no heap allocation.
// A map is allocated lazily only when more than eight keys are stored.
func (c *Context) Set(key string, val any) {
	for i := 0; i < c.storeLen; i++ {
		if c.store[i].k == key {
			c.store[i].v = val
			return
		}
	}
	if c.storeLen < len(c.store) {
		c.store[c.storeLen].k = key
		c.store[c.storeLen].v = val
		c.storeLen++
		return
	}
	c.overflow.Set(key, val)
}

// Get retrieves a value previously stored with [Context.Set].
// Returns nil if the key is not present.
func (c *Context) Get(key string) any {
	for i := 0; i < c.storeLen; i++ {
		if c.store[i].k == key {
			return c.store[i].v
		}
	}
	v, _ := c.overflow.Get(key)
	return v
}

// newContext retrieves a Context from the pool, resets it, and attaches it
// to r.Context() so [FromRequest] works for any code that has only *http.Request.
func newContext(w http.ResponseWriter, r *http.Request) (*Context, *http.Request) {
	c := contextPool.Get().(*Context)
	c.reset(w, r)
	r = r.WithContext(context.WithValue(r.Context(), zenCtxKey{}, c))
	c.Request = r
	return c, r
}

// releaseContext returns the Context to the pool after resetting it.
// It is called by [Router.ServeHTTP] after the handler chain completes.
func releaseContext(c *Context) {
	c.reset(nil, nil)
	contextPool.Put(c)
}

// FromRequest retrieves the *Context stored by [Router.ServeHTTP].
// Returns nil if the request did not pass through a zen Router.
func FromRequest(r *http.Request) *Context {
	if c := r.Context().Value(zenCtxKey{}); c != nil {
		return c.(*Context)
	}
	return nil
}
