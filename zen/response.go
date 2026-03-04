package zen

import (
	"encoding/json"
	"log"
)

// JSON writes a JSON-encoded response with the given HTTP status code.
func (c *Context) JSON(status int, data any) {
	b, err := json.Marshal(data)
	if err != nil {
		// Headers not yet written — we can still send an error status.
		log.Printf("zen: JSON marshal error: %v", err)
		c.Response.WriteHeader(500)
		return
	}
	c.Response.Header().Set("Content-Type", "application/json")
	c.Response.WriteHeader(status)
	if _, err = c.Response.Write(b); err != nil {
		// Headers/status already committed; the best effort is logging.
		log.Printf("zen: JSON write error: %v", err)
	}
}
