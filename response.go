package zen

import (
	"bytes"
	"encoding/json"
	"encoding/xml"
	"log"
	"net/http"
	"sync"
)

// responseBufPool reuses bytes.Buffer instances for response encoding to reduce allocations.
// This is critical for high-throughput servers where JSON/XML responses are common.
var responseBufPool = sync.Pool{New: func() any { return new(bytes.Buffer) }}

// JSON encodes data as JSON and writes it to the response with the given HTTP status.
// The response Content-Type header is automatically set to "application/json".
// Uses a buffer pool to minimize allocations for each response.
//
// If encoding fails, logs the error and sends a 500 error response instead.
// Write errors are logged but not returned (they indicate connection issues).
//
// Example:
//
//	c.JSON(http.StatusOK, map[string]any{
//	    "id":   user.ID,
//	    "name": user.Name,
//	})
//
//	c.JSON(http.StatusCreated, user)
//
//	c.JSON(http.StatusBadRequest, map[string]string{
//	    "error": "invalid email format",
//	})
func (c *Context) JSON(status int, data any) {
	buf := responseBufPool.Get().(*bytes.Buffer)
	buf.Reset()

	if err := json.NewEncoder(buf).Encode(data); err != nil {
		responseBufPool.Put(buf)
		log.Printf("zen: JSON encode error: %v", err)
		http.Error(c.Response, `{"error":"internal server error"}`, http.StatusInternalServerError)
		return
	}

	c.Response.Header().Set("Content-Type", "application/json")
	c.Response.WriteHeader(status)
	if _, err := c.Response.Write(buf.Bytes()); err != nil {
		log.Printf("zen: response write error: %v", err)
	}

	responseBufPool.Put(buf)
}

// XML encodes data as XML and writes it to the response with the given HTTP status.
// The response Content-Type header is automatically set to "application/xml".
// Uses a buffer pool to minimize allocations for each response.
//
// If encoding fails, logs the error and sends a 500 error response instead.
// Write errors are logged but not returned (they indicate connection issues).
//
// Example:
//
//	type User struct {
//	    XMLName xml.Name `xml:"user"`
//	    ID      int      `xml:"id"`
//	    Name    string   `xml:"name"`
//	}
//
//	c.XML(http.StatusOK, User{ID: 1, Name: "John"})
//
//	c.XML(http.StatusCreated, user)
func (c *Context) XML(status int, data any) {
	buf := responseBufPool.Get().(*bytes.Buffer)
	buf.Reset()

	if err := xml.NewEncoder(buf).Encode(data); err != nil {
		responseBufPool.Put(buf)
		log.Printf("zen: XML encode error: %v", err)
		http.Error(c.Response, "internal server error", http.StatusInternalServerError)
		return
	}

	c.Response.Header().Set("Content-Type", "application/xml")
	c.Response.WriteHeader(status)
	if _, err := c.Response.Write(buf.Bytes()); err != nil {
		log.Printf("zen: response write error: %v", err)
	}

	responseBufPool.Put(buf)
}

// HTML writes an HTML string directly to the response with the given HTTP status.
// The response Content-Type header is automatically set to "text/html; charset=utf-8".
// This method does not perform any encoding or escaping - the HTML should be properly
// formatted and escaped by the caller.
//
// Write errors are logged but not returned (they indicate connection issues).
//
// Example:
//
//	c.HTML(http.StatusOK, "<h1>Hello World</h1>")
//
//	c.HTML(http.StatusOK, `
//	    <!DOCTYPE html>
//	    <html>
//	    <head><title>My App</title></head>
//	    <body><h1>Welcome</h1></body>
//	    </html>
//	`)
func (c *Context) HTML(status int, html string) {
	c.Response.Header().Set("Content-Type", "text/html; charset=utf-8")
	c.Response.WriteHeader(status)
	if _, err := c.Response.Write([]byte(html)); err != nil {
		log.Printf("zen: response write error: %v", err)
	}
}

// String writes a plain text string directly to the response with the given HTTP status.
// The response Content-Type header is automatically set to "text/plain; charset=utf-8".
// This method is useful for returning plain text responses like API keys, tokens, or simple messages.
//
// Write errors are logged but not returned (they indicate connection issues).
//
// Example:
//
//	c.String(http.StatusOK, "Hello World")
//
//	c.String(http.StatusOK, "your-api-key-here")
//
//	c.String(http.StatusUnauthorized, "Invalid credentials")
func (c *Context) String(status int, text string) {
	c.Response.Header().Set("Content-Type", "text/plain; charset=utf-8")
	c.Response.WriteHeader(status)
	if _, err := c.Response.Write([]byte(text)); err != nil {
		log.Printf("zen: response write error: %v", err)
	}
}

// Error writes a plain text error message to the response with the given HTTP status.
// Unlike JSON(), this sends plain text without JSON encoding, similar to http.Error().
// The response Content-Type header is set to "text/plain; charset=utf-8".
//
// This is the preferred method for simple error responses where JSON structure
// is not needed. For structured error responses, use JSON() with a plain map or
// custom struct.
//
// Example:
//
//	c.Error(http.StatusBadRequest, "invalid email format")
//	c.Error(http.StatusUnauthorized, "authentication required")
//	c.Error(http.StatusInternalServerError, "database connection failed")
func (c *Context) Error(status int, message string) {
	http.Error(c.Response, message, status)
}

