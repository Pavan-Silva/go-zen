package zen

import (
	"encoding/json"
	"net/http"
)

// JSON writes a JSON-encoded response with the given HTTP status code.
func (c *Context) JSON(status int, data any) {
	b, err := json.Marshal(data)
	if err != nil {
		http.Error(c.Response, `{"error":"internal server error"}`, http.StatusInternalServerError)
		return
	}
	c.Response.Header().Set("Content-Type", "application/json")
	c.Response.WriteHeader(status)
	if _, err = c.Response.Write(b); err != nil {
		// Client disconnected. Nothing actionable — kernel handles TCP cleanup.
		_ = err
	}
}
