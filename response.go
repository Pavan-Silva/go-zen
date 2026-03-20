package zen

import (
	"bytes"
	"encoding/json"
	"log"
	"net/http"
	"sync"
)

// jsonBufPool reuses encoding buffers across requests to avoid a heap
// allocation on every JSON response. Each buffer is reset before use and
// returned to the pool immediately after write, so pooling is safe
// across goroutines.
//
// Unlike pooling json.Decoder (which has no Reset method), json.Encoder
// writes into the buffer we supply — so we pool the buffer, not the encoder.
var jsonBufPool = sync.Pool{New: func() any { return new(bytes.Buffer) }}

// JSON writes a JSON-encoded response with the given HTTP status code.
//
// Encoding is performed into a pooled buffer to avoid a per-request heap
// allocation. The buffer is returned to the pool immediately after the
// response is written, before any downstream code runs.
//
// If marshaling fails the response is not yet committed, so a proper 500 is
// written and the error is logged — it always indicates a programmer mistake
// (passing an un-serializable type) rather than a client error.
//
// Write errors after the headers are committed indicate a client disconnect;
// they are silently discarded because the kernel already handles TCP cleanup
// and there is no recovery action available at that point.
func (c *Context) JSON(status int, data any) {
	buf := jsonBufPool.Get().(*bytes.Buffer)
	buf.Reset()

	if err := json.NewEncoder(buf).Encode(data); err != nil {
		jsonBufPool.Put(buf)
		// Headers not yet written — send a well-formed error response and log
		// so the developer can identify the offending handler.
		log.Printf("zen: JSON encode error: %v", err)
		http.Error(c.Response, `{"error":"internal server error"}`, http.StatusInternalServerError)
		return
	}

	c.Response.Header().Set("Content-Type", "application/json")
	c.Response.WriteHeader(status)
	_, _ = c.Response.Write(buf.Bytes()) // client disconnect is not actionable

	jsonBufPool.Put(buf)
}
