package zen

import (
	"net/http"
	"net/url"
)

type Context struct {
	Response   http.ResponseWriter
	Request    *http.Request
	queryCache url.Values
}

// reset prepares the context for a new request, or zeros it out when called
// with nil values before returning to the pool. Accepting nil here means the
// pool-return path in adaptContextHandler can call c.reset(nil, nil) instead
// of manually clearing each field — ensuring no field is ever forgotten when
// Context gains new fields in the future.
func (c *Context) reset(w http.ResponseWriter, r *http.Request) {
	c.Response = w
	c.Request = r
	c.queryCache = nil
}
