package zen

import (
	"context"
	"net/http"
	"net/url"
)

type Context struct {
	Response   http.ResponseWriter
	Request    *http.Request
	queryCache url.Values
	Ctx        context.Context
}

// reset prepares the context for a new request.
// We clear the queryCache to prevent data leaking between requests.
func (c *Context) reset(w http.ResponseWriter, r *http.Request) {
	c.Response = w
	c.Request = r
	c.Ctx = r.Context()
	c.queryCache = nil
}
