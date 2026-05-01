package zen

import (
	"fmt"
	"io"
)

// QueryParam returns the first value of a query parameter from the request URL.
// Returns an empty string if the parameter is not present.
//
// Example:
//
//	page := c.QueryParam("page")      // GET /posts?page=2 → "2"
//	search := c.QueryParam("q")       // GET /posts?q=golang → "golang"
//	missing := c.QueryParam("foo")    // GET /posts → ""
//
// Multiple values (e.g., "?item=1&item=2") return only the first value.
// Use c.Request.URL.Query() directly for access to all values.
func (c *Context) QueryParam(key string) string {
	vals := c.Request.URL.Query()[key]
	if len(vals) > 0 {
		return vals[0]
	}
	return ""
}

// Param returns the URL path parameter for the given key using Go 1.22+
// enhanced routing via [http.Request.PathValue].
//
// Given a route registered as "GET /users/{id}", the handler can retrieve
// the captured segment with:
//
//	id := c.Param("id")  // GET /users/42  →  "42"
//
// Returns an empty string if the parameter is not found. Requires Go 1.22+ router.
func (c *Context) Param(key string) string {
	return c.Request.PathValue(key)
}

// Body reads and returns the complete raw request body as a byte slice.
// It is the caller's responsibility to interpret the bytes (e.g. as plain
// text, XML, or a custom binary format).
//
// For JSON payloads prefer [Context.BindJSON], which streams directly and
// runs validation in the same call.
//
// The request body is closed after reading. Close errors are silently
// discarded.
//
// Example:
//
//	data, err := c.Body()
//	if err != nil {
//	    c.Error(http.StatusInternalServerError, "failed to read body")
//	    return
//	}
//	// Process raw bytes, e.g. XML parsing, custom format, etc.
func (c *Context) Body() ([]byte, error) {
	b, err := io.ReadAll(c.Request.Body)
	closeErr := c.Request.Body.Close()
	if err != nil {
		if closeErr != nil {
			return nil, fmt.Errorf("%w; body close error: %v", err, closeErr)
		}
		return nil, err
	}
	if closeErr != nil {
		return nil, closeErr
	}
	return b, nil
}
