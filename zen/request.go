package zen

import (
	"bytes"
	"encoding/json"
	"io"
	"reflect"
	"sync"
)

// bufPool reuses byte buffers across requests to avoid a heap allocation on
// every BindJSON call. Each buffer is reset before use, so pooling is safe
// across goroutines.
//
// This is preferable to pooling json.Decoder directly because json.Decoder
// has no Reset method in the Go standard library — a new decoder must be
// allocated each time. By pooling the read buffer instead, we eliminate the
// larger body-copy allocation while accepting the smaller decoder allocation.
var bufPool = sync.Pool{New: func() any { return new(bytes.Buffer) }}

// BindJSON decodes the request body as JSON into dest, then runs struct
// validation if dest is a pointer to a struct.
//
// Validation rules are read from the "validate" struct tag:
//
//	type CreateUserRequest struct {
//	    Username string `json:"username" validate:"required,alphanum"`
//	    Email    string `json:"email"    validate:"required,email"`
//	}
//
// BindJSON returns the first error encountered — either a decode error or a
// validation error. The caller is responsible for writing an appropriate HTTP
// response on error.
//
// The request body is always closed after reading regardless of outcome.
// Body close errors are silently discarded; a failure to close indicates the
// underlying TCP connection is already gone, making the error non-actionable.
func (c *Context) BindJSON(dest any) error {
	buf := bufPool.Get().(*bytes.Buffer)
	buf.Reset()

	_, err := buf.ReadFrom(c.Request.Body)
	_ = c.Request.Body.Close() // best-effort — TCP disconnect is not actionable

	if err == nil {
		err = json.Unmarshal(buf.Bytes(), dest)
	}

	bufPool.Put(buf)

	if err != nil {
		return err
	}

	val := reflect.ValueOf(dest)
	if val.Kind() == reflect.Ptr {
		val = val.Elem()
	}
	if val.Kind() == reflect.Struct {
		return validatorInstance().Struct(dest)
	}
	return nil
}

// QueryParam returns the first value associated with the given query parameter
// key. If the key is not present, an empty string is returned.
//
// The raw query string is parsed and cached on the first call; subsequent
// calls for the same request read from the cache at no extra cost.
//
//	page    := c.QueryParam("page")  // ?page=2  →  "2"
//	missing := c.QueryParam("x")     // not present  →  ""
func (c *Context) QueryParam(key string) string {
	if c.queryCache == nil {
		c.queryCache = c.Request.URL.Query()
	}
	return c.queryCache.Get(key)
}

// Param returns the URL path parameter for the given key using Go 1.22+
// enhanced routing via [http.Request.PathValue].
//
// Given a route registered as "GET /users/{id}", the handler can retrieve
// the captured segment with:
//
//	id := c.Param("id")  // GET /users/42  →  "42"
func (c *Context) Param(key string) string {
	return c.Request.PathValue(key)
}

// Body reads and returns the complete raw request body as a byte slice.
// It is the caller's responsibility to interpret the bytes (e.g. as plain
// text, XML, or a custom binary format).
//
// For JSON payloads prefer [Context.BindJSON], which reuses a pooled buffer
// and runs validation in the same call.
//
// The request body is closed after reading. Close errors are silently
// discarded for the same reason as in [Context.BindJSON].
func (c *Context) Body() ([]byte, error) {
	b, err := io.ReadAll(c.Request.Body)
	_ = c.Request.Body.Close()
	return b, err
}
