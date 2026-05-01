package zen

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"

	"github.com/Pavan-Silva/go-zen/logger"
)

// responseBufPool reuses bytes.Buffer instances for response encoding to reduce allocations.
// This is critical for high-throughput servers where JSON/XML responses are common.
var responseBufPool = sync.Pool{New: func() any { return new(bytes.Buffer) }}

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
	var err error
	if err = dec.Decode(dest); err != nil {
		closeErr := c.Request.Body.Close()
		if closeErr != nil {
			return fmt.Errorf("%w; body close error: %v", err, closeErr)
		}
		return err
	}

	// Ensure there is no trailing data after a single JSON value.
	if _, err = dec.Token(); err != io.EOF {
		closeErr := c.Request.Body.Close()
		if closeErr != nil {
			return fmt.Errorf("request body must contain only one JSON object; body close error: %v", closeErr)
		}
		if err == nil {
			return fmt.Errorf("request body must contain only one JSON object")
		}
		return fmt.Errorf("request body must contain only one JSON object: %w", err)
	}

	if err = c.Request.Body.Close(); err != nil {
		return err
	}

	return validateStruct(dest)
}

// JSON encodes data as JSON and writes it to the response with the given HTTP status.
// The response Content-Type header is automatically set to "application/json".
// Uses a buffer pool to minimize allocations for each response.
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
	buf := responseBufPool.Get().(*bytes.Buffer)
	buf.Reset()

	if err := json.NewEncoder(buf).Encode(data); err != nil {
		responseBufPool.Put(buf)
		logger.Error("HTTP: JSON encode error: %v", err)
		http.Error(c.Response, `{"error":"internal server error"}`, http.StatusInternalServerError)
		return
	}

	c.Response.Header().Set("Content-Type", "application/json")
	c.Response.WriteHeader(status)
	if _, err := c.Response.Write(buf.Bytes()); err != nil {
		logger.Error("HTTP: response write error: %v", err)
	}

	responseBufPool.Put(buf)
}
