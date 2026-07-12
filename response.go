package zen

import (
	"io"
	"net/http"
	"path/filepath"

	"github.com/Pavan-Silva/go-zen/internal/log"
)

// setContentType sets the Content-Type header using direct map access,
// bypassing the canonicalization overhead of http.Header.Set.
// Only writes when the header is absent, avoiding a heap-allocated
// []string{ct} on every call when the value is already present.
func (c *Ctx) setContentType(ct string) {
	h := c.Response.Header()
	if _, exists := h["Content-Type"]; !exists {
		h["Content-Type"] = []string{ct}
	}
}

// HTML writes an HTML string directly to the response with the given HTTP status.
// The response Content-Type header is automatically set to "text/html; charset=utf-8".
//
// Example:
//
//	c.HTML(http.StatusOK, "<h1>Hello World</h1>")
func (c *Ctx) HTML(status int, html string) {
	c.setContentType("text/html; charset=utf-8")
	c.Response.WriteHeader(status)
	if _, err := io.WriteString(c.Response, html); err != nil {
		log.Error("HTTP: HTML response write error: %v", err)
	}
}

// String writes a plain text string directly to the response with the given HTTP status.
//
// Example:
//
//	c.String(http.StatusOK, "Hello World")
func (c *Ctx) String(status int, text string) {
	c.setContentType("text/plain; charset=utf-8")
	c.Response.WriteHeader(status)
	if _, err := io.WriteString(c.Response, text); err != nil {
		log.Error("HTTP: string response write error: %v", err)
	}
}

// Status writes a response header with no-body and the given HTTP status.
//
// Example:
//
//	c.Status(http.StatusNoContent)
func (c *Ctx) Status(status int) {
	c.Response.WriteHeader(status)
}

// Redirect sends an HTTP redirect to the specified location.
// It sets the Location header and writes the provided redirect status.
//
// Example:
//
//	c.Redirect(http.StatusMovedPermanently, "https://example.com/new")
func (c *Ctx) Redirect(status int, location string) {
	http.Redirect(c.Response, c.Request, location, status)
}

// File sends the requested file as the HTTP response body.
// The content type is inferred automatically and errors are handled by http.ServeFile.
//
// Example:
//
//	c.File("/tmp/report.pdf")
func (c *Ctx) File(filePath string) {
	http.ServeFile(c.Response, c.Request, filePath)
}

// Attachment sends the requested file as a downloadable attachment.
// The Content-Disposition header is set with the provided attachment name.
// If attachmentName is empty, the file base name is used.
//
// Example:
//
//	c.Attachment("/tmp/report.pdf", "monthly-report.pdf")
func (c *Ctx) Attachment(filePath, attachmentName string) {
	if attachmentName == "" {
		attachmentName = filepath.Base(filePath)
	}

	c.Response.Header().Set("Content-Disposition", "attachment; filename=\""+attachmentName+"\"")
	http.ServeFile(c.Response, c.Request, filePath)
}

// Inline sends the requested file to be displayed inline by the browser.
// The Content-Disposition header is set to inline.
//
// Example:
//
//	c.Inline("/tmp/image.png")
func (c *Ctx) Inline(filePath string) {
	c.Response.Header().Set("Content-Disposition", "inline")
	http.ServeFile(c.Response, c.Request, filePath)
}

// Blob writes a byte slice as the response body with the given content type.
// The response Content-Type is set to the provided MIME type.
//
// Example:
//
//	data := []byte("id,name\n1,John Doe")
//	c.Blob(http.StatusOK, "text/csv", data)
func (c *Ctx) Blob(status int, contentType string, data []byte) {
	c.setContentType(contentType)
	c.Response.WriteHeader(status)
	if _, err := c.Response.Write(data); err != nil {
		log.Error("HTTP: blob response write error: %v", err)
	}
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
func (c *Ctx) Stream(status int, contentType string, body io.Reader) error {
	c.setContentType(contentType)
	c.Response.WriteHeader(status)
	if _, err := io.Copy(c.Response, body); err != nil {
		log.Error("HTTP: response stream error: %v", err)
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
func (c *Ctx) Error(status int, message string) {
	http.Error(c.Response, message, status)
}
