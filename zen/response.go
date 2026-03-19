package zen

import (
	"encoding/json"
	"log"
	"net/http"
)

// JSON writes a JSON-encoded response with the given HTTP status code.
//
// If marshaling fails the response is not yet committed, so a proper 500 is
// written and the error is logged — it always indicates a programmer mistake
// (passing an un-serializable type) rather than a client error.
//
// Write errors after the headers are committed indicate a client disconnect;
// they are silently discarded because the kernel already handles TCP cleanup
// and there is no recovery action available at this point.
func (c *Context) JSON(status int, data any) {
	b, err := json.Marshal(data)
	if err != nil {
		// Headers not yet written — send a well-formed error response and log
		// so the developer can identify the offending handler.
		log.Printf("zen: JSON marshal error: %v", err)
		http.Error(c.Response, `{"error":"internal server error"}`, http.StatusInternalServerError)
		return
	}
	c.Response.Header().Set("Content-Type", "application/json")
	c.Response.WriteHeader(status)
	_, _ = c.Response.Write(b) // client disconnect is not actionable
}
