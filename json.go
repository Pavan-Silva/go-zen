package zen

import (
	"encoding/json"
	"fmt"

	"github.com/Pavan-Silva/go-zen/logger"
)

// BindJSON parses the request body as JSON and decodes it into dest.
// Uses json.Decoder which streams directly from the request body.
func (c *Context) BindJSON(dest any) error {
	dec := json.NewDecoder(c.Request.Body)
	if err := dec.Decode(dest); err != nil {
		return fmt.Errorf("JSON decode: %w", err)
	}
	return nil
}

// JSON encodes data as JSON and writes it to the response with the given HTTP status.
// Uses json.Encoder which streams directly to the response writer.
func (c *Context) JSON(status int, data any) {
	c.Response.Header().Set("Content-Type", "application/json")
	c.Response.WriteHeader(status)

	if err := json.NewEncoder(c.Response).Encode(data); err != nil {
		logger.Error("HTTP: JSON encode error: %v", err)
	}
}
