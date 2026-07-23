package zen

import (
	"encoding/xml"

	"github.com/Pavan-Silva/go-zen/internal/log"
)

// XMLSerializer is the interface for XML encoding and decoding.
// Implementations handle serializing Go values to XML for responses
// and deserializing XML request bodies into Go values.
type XMLSerializer interface {
	Serialize(c *Ctx, v any) error
	Deserialize(c *Ctx, v any) error
}

type xmlSerializer struct{}

func (xmlSerializer) Serialize(c *Ctx, v any) error {
	return xml.NewEncoder(c.Response).Encode(v)
}

func (xmlSerializer) Deserialize(c *Ctx, v any) error {
	return xml.NewDecoder(c.Request.Body).Decode(v)
}

// XML encodes data as XML and writes it to the response with the given HTTP status code.
func (c *Ctx) XML(status int, data any) {
	c.setContentType("application/xml")
	c.Response.WriteHeader(status)

	if err := c.engine.XMLSerializer.Serialize(c, data); err != nil {
		log.Error("HTTP: XML encode error: %v", err)
	}
}

// BindXML decodes the request body as XML into dest.
func (c *Ctx) BindXML(dest any) error {
	if err := c.engine.XMLSerializer.Deserialize(c, dest); err != nil {
		return err
	}
	if c.engine.autoValidate && c.engine.validator != nil {
		return c.engine.validator.Validate(dest)
	}
	return nil
}
