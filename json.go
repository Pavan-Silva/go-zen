package zen

import "encoding/json"

// JSONSerializer is the interface for JSON encoding and decoding.
// Implementations handle serializing Go values to JSON for responses
// and deserializing JSON request bodies into Go values.
type JSONSerializer interface {
	Serialize(c *Ctx, v any, indent string) error
	Deserialize(c *Ctx, v any) error
}

type jsonSerializer struct{}

func (jsonSerializer) Serialize(c *Ctx, v any, indent string) error {
	enc := json.NewEncoder(c.Response)
	if indent != "" {
		enc.SetIndent("", indent)
	}
	return enc.Encode(v)
}

func (jsonSerializer) Deserialize(c *Ctx, v any) error {
	if c.Request.Body == nil {
		return nil
	}
	return json.NewDecoder(c.Request.Body).Decode(v)
}

// JSON encodes data as JSON and streams it straight to the response writer.
func (c *Ctx) JSON(status int, data any) {
	c.writeJSON(status, data, "")
}

// JSONPretty encodes data as indented JSON for human-readable responses.
func (c *Ctx) JSONPretty(status int, data any) {
	c.writeJSON(status, data, "  ")
}

// writeJSON sets the JSON content type and streams data to the response writer.
func (c *Ctx) writeJSON(status int, data any, indent string) {
	c.writeResponse("JSON encode", status, contentTypeJSON, func() error {
		return c.engine.JSONSerializer.Serialize(c, data, indent)
	})
}

// BindJSON decodes the request body as JSON into dest.
func (c *Ctx) BindJSON(dest any) error {
	if err := c.engine.JSONSerializer.Deserialize(c, dest); err != nil {
		return err
	}
	return c.engine.validateIfEnabled(dest)
}
