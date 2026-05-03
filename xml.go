package zen

import (
	"encoding/xml"
	"fmt"
	"io"
	"net/http"

	"github.com/Pavan-Silva/go-zen/logger"
)

// BindXML parses the request body as XML and decodes it into dest.
// If dest is a struct, it also runs struct validation using the registered validator.
// The request body is closed after reading (errors are logged but not returned).
//
// Returns an error if:
// - The XML is malformed
// - dest is not a pointer to a struct for validation
// - Validation fails (if enabled)
//
// Example:
//
//	type SignupRequest struct {
//	    Email string `xml:"email"`
//	    Age   int    `xml:"age"`
//	}
//	var req SignupRequest
//	if err := c.BindXML(&req); err != nil {
//	    c.Error(http.StatusBadRequest, err.Error())
//	    return
//	}
func (c *Context) BindXML(dest any) error {
	dec := xml.NewDecoder(c.Request.Body)

	if err := dec.Decode(dest); err != nil {
		c.Request.Body.Close()
		return fmt.Errorf("XML decode: %w", err)
	}

	// Ensure there is no trailing data after a single XML element.
	if _, err := dec.Token(); err != io.EOF {
		c.Request.Body.Close()
		return fmt.Errorf("request body must contain only one XML element: %w", err)
	}

	if err := c.Request.Body.Close(); err != nil {
		return err
	}

	return validateStruct(dest)
}

// XML encodes data as XML and writes it to the response with the given HTTP status.
// The response Content-Type header is automatically set to "application/xml".
//
// Uses xml.Encoder which streams directly to the response writer instead of
// marshaling to memory first, improving performance for large payloads.
//
// The status code is not sent until the first write, so if encoding fails,
// the status can still be changed to 500.
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
	c.Response.Header().Set("Content-Type", "application/xml")

	// Use delayedStatusWriter so we can change status to 500 if encode fails.
	resp := c.Response
	c.SetResponse(&delayedStatusWriter{ResponseWriter: resp, status: status})
	defer c.SetResponse(resp)

	enc := xml.NewEncoder(c.Response)
	if err := enc.Encode(data); err != nil {
		logger.Error("HTTP: XML encode error: %v", err)
		c.SetResponse(resp) // restore original so we can write error
		resp.WriteHeader(http.StatusInternalServerError)
		_, _ = resp.Write([]byte(`<error>internal server error</error>`))
	}
}
