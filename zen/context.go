package zen

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"sync"
)

var bufPool = sync.Pool{
	New: func() interface{} {
		return new(bytes.Buffer)
	},
}

type Context struct {
	Response   http.ResponseWriter
	Request    *http.Request
	queryCache url.Values
}

// JSON writes the given data as JSON to the response with the status code.
// It borrows a buffer from the pool to avoid the per-request bufio.Writer
// allocation that json.NewEncoder carries. Content-Type is only set if not
// already present to avoid the MIMEHeader.Set allocation on repeat calls.
func (c *Context) JSON(status int, data interface{}) error {
	h := c.Response.Header()
	if h.Get("Content-Type") == "" {
		h.Set("Content-Type", "application/json")
	}
	c.Response.WriteHeader(status)

	buf := bufPool.Get().(*bytes.Buffer)
	buf.Reset()
	defer bufPool.Put(buf)

	if err := json.NewEncoder(buf).Encode(data); err != nil {
		return err
	}
	_, err := buf.WriteTo(c.Response)
	return err
}

// BindJSON decodes a JSON request body into dest. It borrows a buffer from
// the pool to read the body, then unmarshals from the buffer — avoiding the
// bufio.Reader allocation that json.NewDecoder allocates on every call.
func (c *Context) BindJSON(dest interface{}) error {
	defer c.Request.Body.Close()

	buf := bufPool.Get().(*bytes.Buffer)
	buf.Reset()
	defer bufPool.Put(buf)

	if _, err := buf.ReadFrom(c.Request.Body); err != nil {
		return err
	}
	return json.Unmarshal(buf.Bytes(), dest)
}

// FormValue retrieves a form value (works for both urlencoded forms and multipart).
func (c *Context) FormValue(key string) string {
	c.Request.ParseForm()
	return c.Request.FormValue(key)
}

// QueryParam returns a URL query parameter, parsing the query string only
// once per request and caching the result.
func (c *Context) QueryParam(key string) string {
	if c.queryCache == nil {
		c.queryCache, _ = url.ParseQuery(c.Request.URL.RawQuery)
	}
	return c.queryCache.Get(key)
}

// Param is a placeholder for named route parameters.
func (c *Context) Param(key string) string {
	return ""
}

// Body returns the raw request body as bytes.
func (c *Context) Body() ([]byte, error) {
	defer c.Request.Body.Close()
	return io.ReadAll(c.Request.Body)
}