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
// Returns an error if the body is not valid JSON or cannot be decoded into dest.
//
// Example:
//
//	var req CreateUserRequest
//	if err := c.BindJSON(&req); err != nil {
//	    c.Error(http.StatusBadRequest, err.Error())
//	    return
//	}
//	if err := zen.Validate(&req); err != nil {
//	    c.Error(http.StatusBadRequest, err.Error())
//	    return
//	}
func (c *Context) BindJSON(dest any) error {
	dec := json.NewDecoder(c.Request.Body)
	if err := dec.Decode(dest); err != nil {
		return fmt.Errorf("JSON decode: %w", err)
	}
	return nil
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
func (c *Context) JSON(status int, data any) {
	c.Response.Header().Set("Content-Type", "application/json")
	c.Response.WriteHeader(status)

	if err := json.NewEncoder(c.Response).Encode(data); err != nil {
		logger.Error("HTTP: JSON encode error: %v", err)
	}
}
