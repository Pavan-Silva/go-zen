package zen

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
)

const sseContentType = "text/event-stream; charset=utf-8"

// ErrFlusherUnsupported is returned by SSEvent when the ResponseWriter does not implement http.Flusher.
var ErrFlusherUnsupported = errors.New("sse: SSE requires http.Flusher")

// Pre-allocated static byte slices to completely eliminate runtime string-to-byte allocations.
var (
	prefixEvent   = []byte("event: ")
	prefixData    = []byte("data: ")
	tokenNL       = []byte("\n")
	tokenDoubleNL = []byte("\n\n")
)

// SSEvent writes a Server-Sent Event to the client.
// It sets required SSE headers and flushes the response.
func (c *Ctx) SSEvent(event string, data any) error {
	// Fast Path: Check if response implements flusher directly
	flusher, ok := c.Response.(http.Flusher)
	if !ok {
		return ErrFlusherUnsupported
	}

	headers := c.Response.Header()
	if _, exists := headers["Content-Type"]; !exists {
		headers["Content-Type"] = []string{sseContentType}
	}
	if _, exists := headers["Cache-Control"]; !exists {
		headers["Cache-Control"] = []string{"no-cache"}
	}
	if _, exists := headers["Connection"]; !exists {
		headers["Connection"] = []string{"keep-alive"}
	}
	if _, exists := headers["X-Accel-Buffering"]; !exists {
		headers["X-Accel-Buffering"] = []string{"no"}
	}

	w := c.Response

	// Write Event Header if present
	if event != "" {
		if _, err := w.Write(prefixEvent); err != nil {
			return err
		}
		// Clean out carriage returns safely without rewriting string allocations
		for i := 0; i < len(event); i++ {
			char := event[i]
			if char != '\r' && char != '\n' {
				if _, err := w.Write([]byte{char}); err != nil {
					return err
				}
			}
		}
		if _, err := w.Write(tokenNL); err != nil {
			return err
		}
	}

	// Write Data Payload Block
	if err := c.writeSSEData(w, data); err != nil {
		return err
	}

	flusher.Flush()
	return nil
}

// writeSSEData streams data fragments directly into the response writer socket buffer.
func (c *Ctx) writeSSEData(w http.ResponseWriter, data any) error {
	switch value := data.(type) {
	case string:
		return writeBytesSSE(w, StringToBytes(value))
	case []byte:
		return writeBytesSSE(w, value)
	default:
		// Fallback to serialization for complex maps/structs
		if _, err := w.Write(prefixData); err != nil {
			return err
		}
		// Use configured serializer path or standard marshal
		b, err := json.Marshal(value)
		if err != nil {
			return err
		}
		if _, err := w.Write(b); err != nil {
			return err
		}
		if _, err := w.Write(tokenDoubleNL); err != nil {
			return err
		}
	}
	return nil
}

// writeBytesSSE processes multi-line data blocks smoothly using direct byte segment spans.
func writeBytesSSE(w http.ResponseWriter, b []byte) error {
	if len(b) == 0 {
		_, err := w.Write(tokenDoubleNL)
		return err
	}

	remaining := b
	for len(remaining) > 0 {
		if _, err := w.Write(prefixData); err != nil {
			return err
		}

		// Look for standard newline boundaries across the remaining raw bytes
		idx := bytes.IndexByte(remaining, '\n')
		if idx == -1 {
			// Write the final chunk
			cleanWrite(w, remaining)
			if _, err := w.Write(tokenDoubleNL); err != nil {
				return err
			}
			break
		}

		// Write up to the newline token slice bounds
		cleanWrite(w, remaining[:idx])
		if _, err := w.Write(tokenNL); err != nil {
			return err
		}
		remaining = remaining[idx+1:]
	}
	return nil
}

// cleanWrite strips out trailing carriage returns ('\r') inline during raw stream flush.
func cleanWrite(w http.ResponseWriter, b []byte) {
	if len(b) > 0 && b[len(b)-1] == '\r' {
		_, _ = w.Write(b[:len(b)-1])
		return
	}
	_, _ = w.Write(b)
}
