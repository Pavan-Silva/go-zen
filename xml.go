package zen

import (
	"encoding/xml"
	"fmt"

	"github.com/Pavan-Silva/go-zen/logger"
)

// BindXML parses the request body as XML and decodes it into dest.
//
// NOTE: This does NOT auto-validate (matching Gin/Echo behavior).
// Call c.Validate(dest) separately if you want struct validation.
//
// Returns an error if:
// - The XML is malformed
// - dest is not a pointer to a struct for validation
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
//	if err := c.Validate(&req); err != nil {
//	    c.Error(http.StatusBadRequest, err.Error())
//	    return
//	}
func (c *Context) BindXML(dest any) error {
	dec := xml.NewDecoder(c.Request.Body)

	if err := dec.Decode(dest); err != nil {
		return fmt.Errorf("XML decode: %w", err)
	}

	return nil
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
	c.Response.WriteHeader(status)

	if err := xml.NewEncoder(c.Response).Encode(data); err != nil {
		logger.Error("HTTP: XML encode error: %v", err)
	}
}
