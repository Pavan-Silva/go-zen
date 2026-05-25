package zen

import (
	"encoding/xml"
	"fmt"

	"github.com/Pavan-Silva/go-zen/logger"
)

// BindXML parses the request body as XML and decodes it into dest.
// Uses xml.NewDecoder which streams directly from the request body.
//
// Automatically runs struct validation after decode if a Validator is configured.
//
// Returns an error if the body is not valid XML, cannot be decoded, or
// struct validation fails.
//
// Example:
//
//	var req CreateUserRequest
//	if err := c.BindXML(&req); err != nil {
//	    c.Error(http.StatusBadRequest, err.Error())
//	    return
//	}
func (c *Ctx) BindXML(dest any) error {
	dec := xml.NewDecoder(c.Request.Body)
	if err := dec.Decode(dest); err != nil {
		return fmt.Errorf("XML decode: %w", err)
	}
	return Validate(dest)
}

// XML encodes data as XML and writes it to the response with the given HTTP status code.
// Uses xml.NewEncoder which streams directly to the response writer.
//
// The response Content-Type header is automatically set to "application/xml".
// Write errors are logged but not returned (they indicate connection issues).
//
// Example:
//
//	type Response struct {
//	    XMLName xml.Name `xml:"response"`
//	    Message string   `xml:"message"`
//	}
//	c.XML(http.StatusOK, Response{Message: "hello"})
func (c *Ctx) XML(status int, data any) {
	c.setContentType("application/xml")
	c.Response.WriteHeader(status)
	if err := xml.NewEncoder(c.Response).Encode(data); err != nil {
		logger.Error("HTTP: XML encode error: %v", err)
	}
}
