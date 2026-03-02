package zen

import (
	"bytes"
	"encoding/json"
	"log"
	"net/http"
	"sync"
)

const maxPooledBufferSize = 64 * 1024 // 64 KB

var bufPool = sync.Pool{
	New: func() any { return new(bytes.Buffer) },
}

func (c *Context) JSON(status int, data any) {
	buf := bufPool.Get().(*bytes.Buffer)
	buf.Reset()
	defer func() {
		if buf.Cap() <= maxPooledBufferSize {
			bufPool.Put(buf)
		}
	}()

	// Encode to buffer first to catch errors before WriteHeader
	if err := json.NewEncoder(buf).Encode(data); err != nil {
		log.Printf("zen: JSON encode error: %v", err)
		c.Response.WriteHeader(http.StatusInternalServerError)
		return
	}

	c.Response.Header().Set("Content-Type", "application/json")
	c.Response.WriteHeader(status)
	_, _ = buf.WriteTo(c.Response)
}
