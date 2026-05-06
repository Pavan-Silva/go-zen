package zen

import (
	"encoding/xml"
	"fmt"

	"github.com/Pavan-Silva/go-zen/logger"
)

// BindXML parses the request body as XML and decodes it into dest.
// Uses xml.Decoder which streams directly from the request body.
func (c *Context) BindXML(dest any) error {
	dec := xml.NewDecoder(c.Request.Body)
	if err := dec.Decode(dest); err != nil {
		return fmt.Errorf("XML decode: %w", err)
	}
	return nil
}

// XML encodes data as XML and writes it to the response with the given HTTP status.
// Uses xml.Encoder which streams directly to the response writer.
func (c *Context) XML(status int, data any) {
	c.Response.Header().Set("Content-Type", "application/xml")
	c.Response.WriteHeader(status)

	if err := xml.NewEncoder(c.Response).Encode(data); err != nil {
		logger.Error("HTTP: XML encode error: %v", err)
	}
}
