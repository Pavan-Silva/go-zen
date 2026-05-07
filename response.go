package zen

import (
	"fmt"
	"io"
	"net/http"
	"path/filepath"

	"github.com/Pavan-Silva/go-zen/logger"
)

// writeResponse sets content type, status, writes body, and logs errors.
// Consolidates the repeated response writing pattern across methods.
func writeResponse(c *Context, status int, contentType string, body []byte) {
	c.Response.Header().Set("Content-Type", contentType)
	c.Response.WriteHeader(status)
	if _, err := c.Response.Write(body); err != nil {
		logger.Error("HTTP: response write error: %v", err)
	}
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
	writeResponse(c, status, "text/html; charset=utf-8", []byte(html))
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
	writeResponse(c, status, "text/plain; charset=utf-8", []byte(text))
}

// NoContent writes a response with no body and the given HTTP status.
// This is useful for responses like 204 No Content or 202 Accepted.
//
// Example:
//
//	c.NoContent(http.StatusNoContent)
func (c *Context) NoContent(status int) {
	c.Response.WriteHeader(status)
}

// Redirect sends an HTTP redirect to the specified location.
// It sets the Location header and writes the provided redirect status.
//
// Example:
//
//	c.Redirect(http.StatusMovedPermanently, "https://example.com/new")
func (c *Context) Redirect(status int, location string) {
	http.Redirect(c.Response, c.Request, location, status)
}

// File sends the requested file as the HTTP response body.
// The content type is inferred automatically and errors are handled by http.ServeFile.
//
// Example:
//
//	c.File("/tmp/report.pdf")
func (c *Context) File(filePath string) {
	http.ServeFile(c.Response, c.Request, filePath)
}

// Attachment sends the requested file as a downloadable attachment.
// The Content-Disposition header is set with the provided attachment name.
// If attachmentName is empty, the file base name is used.
//
// Example:
//
//	c.Attachment("/tmp/report.pdf", "monthly-report.pdf")
func (c *Context) Attachment(filePath, attachmentName string) {
	if attachmentName == "" {
		attachmentName = filepath.Base(filePath)
	}
	c.Response.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", attachmentName))
	http.ServeFile(c.Response, c.Request, filePath)
}

// Inline sends the requested file to be displayed inline by the browser.
// The Content-Disposition header is set to inline.
//
// Example:
//
//	c.Inline("/tmp/image.png")
func (c *Context) Inline(filePath string) {
	c.Response.Header().Set("Content-Disposition", "inline")
	http.ServeFile(c.Response, c.Request, filePath)
}

// Blob writes a byte slice as the response body with the given content type.
// The response Content-Type is set to the provided MIME type.
// Write errors are logged but not returned (they indicate connection issues).
//
// Example:
//
//	data := []byte("id,name\n1,John Doe")
//	c.Blob(http.StatusOK, "text/csv", data)
func (c *Context) Blob(status int, contentType string, data []byte) error {
	writeResponse(c, status, contentType, data)
	return nil
}

// Stream copies data from an io.Reader to the response body with the given content type.
// The response Content-Type is set to the provided MIME type.
//
// Example:
//
//	f, err := os.Open("/tmp/image.png")
//	if err != nil {
//	    return err
//	}
//	defer f.Close()
//	return c.Stream(http.StatusOK, "image/png", f)
func (c *Context) Stream(status int, contentType string, body io.Reader) error {
	c.Response.Header().Set("Content-Type", contentType)
	c.Response.WriteHeader(status)
	if _, err := io.Copy(c.Response, body); err != nil {
		logger.Error("HTTP: response stream error: %v", err)
		return err
	}
	return nil
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
