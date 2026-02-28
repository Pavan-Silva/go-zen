// Package zen defines a lightweight Context type and handler adapter used
// by the zen micro-framework. It lives alongside zen/zen.go but is kept in the
// same package for simplicity.

package zen

import (
    "encoding/json"
    "io"
    "net/http"
)

// Context wraps http.ResponseWriter and *http.Request, providing helpers
// for parsing and responding. It is deliberately minimal and thin.

type Context struct {
    Response http.ResponseWriter
    Request  *http.Request
}

// JSON writes the given data as JSON to the response with the status code.
func (c *Context) JSON(status int, data interface{}) error {
    c.Response.Header().Set("Content-Type", "application/json")
    c.Response.WriteHeader(status)
    return json.NewEncoder(c.Response).Encode(data)
}

// BindJSON decodes JSON request body into dest.
func (c *Context) BindJSON(dest interface{}) error {
    defer c.Request.Body.Close()
    return json.NewDecoder(c.Request.Body).Decode(dest)
}

// FormValue retrieves a form value (works for both urlencoded forms and multipart).
func (c *Context) FormValue(key string) string {
    // ensure form parsed
    c.Request.ParseForm()
    return c.Request.FormValue(key)
}

// QueryParam is shorthand for url query parameter.
func (c *Context) QueryParam(key string) string {
    return c.Request.URL.Query().Get(key)
}

// Param currently a no-op because we're relying on net/http mux; routing libraries
// that support named parameters can extend this. This is left as a placeholder.
func (c *Context) Param(key string) string {
    return ""
}

// Body returns the raw request body as bytes. It reads and closes the body.
func (c *Context) Body() ([]byte, error) {
    defer c.Request.Body.Close()
    return io.ReadAll(c.Request.Body)
}
