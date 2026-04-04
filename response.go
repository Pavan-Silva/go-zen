package zen

import (
	"bytes"
	"encoding/json"
	"log"
	"net/http"
	"sync"
)

var jsonBufPool = sync.Pool{New: func() any { return new(bytes.Buffer) }}

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
