package zen

import (
	"encoding/json"
	"log"
)

func (c *Context) JSON(status int, data any) {
	c.Response.Header().Set("Content-Type", "application/json")
	c.Response.WriteHeader(status)
	if err := json.NewEncoder(c.Response).Encode(data); err != nil {
		// At this point headers/status are already committed; best effort is logging.
		log.Printf("zen: JSON encode error: %v", err)
	}
}
