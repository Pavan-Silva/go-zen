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
		overflow: make(map[string]any, 4),
	}
}

type zenCtxKey struct{}

type kv struct {
	k string
	v any
}

const inlineSlots = 8

type Context struct {
	Response   http.ResponseWriter
	Request    *http.Request
	queryCache map[string]string
	store      [inlineSlots]kv
	storeLen   int
	overflow   map[string]any
}

func (c *Context) reset(w http.ResponseWriter, r *http.Request) {
	c.Response = w
	c.Request = r
	c.queryCache = nil
	c.storeLen = 0
	for k := range c.overflow {
		delete(c.overflow, k)
	}
}

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
	c.overflow[key] = val
}

func (c *Context) Get(key string) any {
	for i := 0; i < c.storeLen; i++ {
		if c.store[i].k == key {
			return c.store[i].v
		}
	}
	v, _ := c.overflow[key]
	return v
}

func newContext(w http.ResponseWriter, r *http.Request) (*Context, *http.Request) {
	c := contextPool.Get().(*Context)
	c.reset(w, r)
	r = r.WithContext(context.WithValue(r.Context(), zenCtxKey{}, c))
	c.Request = r
	return c, r
}

func releaseContext(c *Context) {
	c.reset(nil, nil)
	contextPool.Put(c)
}

func FromRequest(r *http.Request) *Context {
	if c := r.Context().Value(zenCtxKey{}); c != nil {
		return c.(*Context)
	}
	return nil
}
