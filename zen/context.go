package zen

import (
	"net/http"
	"net/url"
	"sync"
)

// contextPool reuses Context objects across requests to avoid per-request heap allocations.
var contextPool = sync.Pool{
	New: func() any { return &Context{} },
}

type Context struct {
	Response   http.ResponseWriter
	Request    *http.Request
	queryCache url.Values
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
}

// adaptHandler turns a zen-style handler into a standard http.Handler.
// It is used at route registration time and reuses Context instances via contextPool.
func adaptHandler(handler func(*Context)) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c := contextPool.Get().(*Context)
		c.reset(w, r)
		handler(c)
		// Nil out references before returning to pool to avoid retaining
		// request/response pointers across requests.
		c.reset(nil, nil)
		contextPool.Put(c)
	})
}
