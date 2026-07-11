package zen

import (
	"encoding/json"
	"fmt"

	"github.com/Pavan-Silva/go-zen/logger"
)

// BindJSON parses the request body as JSON and decodes it into dest.
// Uses json.NewDecoder which streams directly from the request body, avoiding
// intermediate byte slice allocations.
//
// Automatically runs struct validation after decode if a Validator is configured.
func (c *Ctx) BindJSON(dest any) error {
	dec := json.NewDecoder(c.Request.Body)
	if err := dec.Decode(dest); err != nil {
		return fmt.Errorf("JSON decode: %w", err)
	}

	if autoValidateOn() {
		return Validate(dest)
	}
	return nil
}

// JSON encodes data as JSON and streams it straight to the response writer.
//
// Sets the "Content-Type" header to "application/json" automatically.
// Write errors are handled and logged safely via internal system loggers.
func (c *Ctx) JSON(status int, data any) {
	c.setContentType("application/json")
	c.Response.WriteHeader(status)

	if err := json.NewEncoder(c.Response).Encode(data); err != nil {
		logger.Error("HTTP: JSON encode error: %v", err)
	}
}

// JSONPretty encodes data as indented JSON for human-readable responses.
// Uses two-space indentation. For compact JSON, use JSON() instead.
func (c *Ctx) JSONPretty(status int, data any) {
	c.setContentType("application/json")
	c.Response.WriteHeader(status)

	enc := json.NewEncoder(c.Response)
	enc.SetIndent("", "  ")
	if err := enc.Encode(data); err != nil {
		logger.Error("HTTP: JSON encode error: %v", err)
	}
}
