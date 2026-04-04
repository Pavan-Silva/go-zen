package zen

import (
	"bytes"
	"encoding/json"
	"log"
	"net/http"
	"sync"
)

// jsonBufPool reuses bytes.Buffer instances for JSON encoding to reduce allocations.
// This is critical for high-throughput servers where JSON responses are common.
var jsonBufPool = sync.Pool{New: func() any { return new(bytes.Buffer) }}

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
	buf := jsonBufPool.Get().(*bytes.Buffer)
	buf.Reset()

	if err := json.NewEncoder(buf).Encode(data); err != nil {
		jsonBufPool.Put(buf)
		log.Printf("zen: JSON encode error: %v", err)
		http.Error(c.Response, `{"error":"internal server error"}`, http.StatusInternalServerError)
		return
	}

	c.Response.Header().Set("Content-Type", "application/json")
	c.Response.WriteHeader(status)
	if _, err := c.Response.Write(buf.Bytes()); err != nil {
		log.Printf("zen: response write error: %v", err)
	}

	jsonBufPool.Put(buf)
}
