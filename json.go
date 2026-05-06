package zen

import (
	"encoding/json"
	"fmt"
	"io"

	"github.com/Pavan-Silva/go-zen/logger"
)

// BindJSON parses the request body as JSON and decodes it into dest.
//
// NOTE: This does NOT auto-validate (matching Gin/Echo behavior).
// Call c.Validate(dest) separately if you want struct validation.
//
// Returns an error if:
// - The JSON is malformed
//
// Example:
//
//	type SignupRequest struct {
//	    Email string `json:"email" validate:"required,email"`
//	    Age   int    `json:"age" validate:"required,gte=18"`
//	}
//	var req SignupRequest
//	if err := c.BindJSON(&req); err != nil {
//	    c.Error(http.StatusBadRequest, err.Error())
//	    return
//	}
//	if err := c.Validate(&req); err != nil {
//	    c.Error(http.StatusBadRequest, err.Error())
//	    return
//	}
func (c *Context) BindJSON(dest any) error {
	dec := json.NewDecoder(c.Request.Body)

	if err := dec.Decode(dest); err != nil {
		return fmt.Errorf("JSON decode: %w", err)
	}

	// Ensure there is no trailing data after a single JSON value.
	if _, err := dec.Token(); err != io.EOF {
		return fmt.Errorf("request body must contain only one JSON object: %w", err)
	}

	return nil
}

// JSON encodes data as JSON and writes it to the response with the given HTTP status.
// The response Content-Type header is automatically set to "application/json".
//
// Uses json.Encoder which streams directly to the response writer instead of
// marshaling to memory first, improving performance for large payloads.
//
// If encoding fails, logs the error and the response may contain partial JSON.
// Write errors are logged but not returned (they indicate connection issues).
//
// Example:
//
//	c.JSON(http.StatusOK, map[string]any{
//	    "id":   user.ID,
//	    "name": user.Name,
//	})
//
//	c.JSON(http.StatusCreated, user)
//
//	c.JSON(http.StatusBadRequest, map[string]string{
//	    "error": "invalid email format",
//	})
func (c *Context) JSON(status int, data any) {
	c.Response.Header().Set("Content-Type", "application/json")
	c.Response.WriteHeader(status)

	enc := json.NewEncoder(c.Response)
	if err := enc.Encode(data); err != nil {
		logger.Error("HTTP: JSON encode error: %v", err)
	}
}
