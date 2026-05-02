package zen

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/Pavan-Silva/go-zen/logger"
)

// BindJSON parses the request body as JSON and decodes it into dest.
// If dest is a struct, it also runs struct validation using the registered validator.
// The request body is closed after reading (errors are logged but not returned).
//
// Returns an error if:
// - The JSON is malformed
// - dest is not a pointer to a struct for validation
// - Validation fails (if enabled)
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
func (c *Context) BindJSON(dest any) error {
	dec := json.NewDecoder(c.Request.Body)

	if err := dec.Decode(dest); err != nil {
		c.Request.Body.Close()
		return fmt.Errorf("JSON decode: %w", err)
	}

	// Ensure there is no trailing data after a single JSON value.
	if _, err := dec.Token(); err != io.EOF {
		c.Request.Body.Close()
		return fmt.Errorf("request body must contain only one JSON object: %w", err)
	}

	if err := c.Request.Body.Close(); err != nil {
		return err
	}

	return validateStruct(dest)
}

// JSON encodes data as JSON and writes it to the response with the given HTTP status.
// The response Content-Type header is automatically set to "application/json".
//
// If encoding fails, logs the error and sends a 500 error response instead.
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
	b, err := json.Marshal(data)
	if err != nil {
		logger.Error("HTTP: JSON encode error: %v", err)
		http.Error(c.Response, `{"error":"internal server error"}`, http.StatusInternalServerError)
		return
	}

	c.Response.Header().Set("Content-Type", "application/json")
	c.Response.WriteHeader(status)
	if _, err := c.Response.Write(b); err != nil {
		logger.Error("HTTP: response write error: %v", err)
	}
}
