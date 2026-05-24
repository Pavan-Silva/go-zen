package zen

import (
	"encoding/json"
	"fmt"

	"github.com/Pavan-Silva/go-zen/logger"
)

// BindJSON parses the request body as JSON and decodes it into dest.
// Uses json.NewDecoder which streams directly from the request body, avoiding
// an intermediate byte slice allocation (unlike json.Unmarshal).
//
// Automatically runs struct validation after decode if a Validator is configured.
// To skip validation, use SetValidator(nil).
//
// Returns an error if the body is not valid JSON, cannot be decoded, or
// struct validation fails.
//
// Example:
//
//	var req CreateUserRequest
//	if err := c.BindJSON(&req); err != nil {
//	    c.Error(http.StatusBadRequest, err.Error())
//	    return
//	}
func (c *Ctx) BindJSON(dest any) error {
	dec := json.NewDecoder(c.Request.Body)
	if err := dec.Decode(dest); err != nil {
		return fmt.Errorf("JSON decode: %w", err)
	}
	return Validate(dest)
}

// JSON encodes data as JSON and writes it to the response with the given HTTP status code.
// Uses json.NewEncoder which streams directly to the response writer, avoiding an
// intermediate byte slice allocation (unlike json.Marshal).
//
// The response Content-Type header is automatically set to "application/json".
// Write errors are logged but not returned (they indicate connection issues).
//
// Example:
//
//	c.JSON(http.StatusOK, map[string]string{"message": "hello"})
//	c.JSON(http.StatusOK, user)
func (c *Ctx) JSON(status int, data any) {
	c.Response.Header().Set("Content-Type", "application/json")
	c.Response.WriteHeader(status)

	if err := json.NewEncoder(c.Response).Encode(data); err != nil {
		logger.Error("HTTP: JSON encode error: %v", err)
	}
}
