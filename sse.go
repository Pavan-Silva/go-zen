package zen

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"
)

const contentType = "text/event-stream; charset=utf-8"

var ErrFlusherUnsupported = errors.New("sse: SSE requires http.Flusher")

// Send writes a Server-Sent Event to the client.
// It sets required SSE headers and flushes the response.
func (c *Context) SSEvent(event string, data any) error {
	flusher, ok := c.Response.(http.Flusher)
	if !ok {
		return ErrFlusherUnsupported
	}

	headers := c.Response.Header()
	// Set SSE headers only if not already configured by user code.
	if !hasSSEContentType(headers.Get("Content-Type")) {
		headers.Set("Content-Type", contentType)
	}
	if headers.Get("Cache-Control") == "" {
		headers.Set("Cache-Control", "no-cache")
	}
	if headers.Get("Connection") == "" {
		headers.Set("Connection", "keep-alive")
	}
	if headers.Get("X-Accel-Buffering") == "" {
		headers.Set("X-Accel-Buffering", "no")
	}

	payload, err := encodeSSEData(data)
	if err != nil {
		return err
	}

	w := c.Response
	if event != "" {
		// Event names are a single line in SSE.
		event = strings.ReplaceAll(event, "\r", "")
		event = strings.ReplaceAll(event, "\n", "")
		if _, err := w.Write([]byte("event: " + event + "\n")); err != nil {
			return err
		}
	}

	if _, err := w.Write([]byte("data: " + payload + "\n\n")); err != nil {
		return err
	}

	flusher.Flush()
	return nil
}

func encodeSSEData(data any) (string, error) {
	switch value := data.(type) {
	case string:
		value = strings.ReplaceAll(value, "\r\n", "\n")
		value = strings.ReplaceAll(value, "\r", "\n")
		return strings.ReplaceAll(value, "\n", "\ndata: "), nil
	case []byte:
		s := strings.ReplaceAll(string(value), "\r\n", "\n")
		s = strings.ReplaceAll(s, "\r", "\n")
		return strings.ReplaceAll(s, "\n", "\ndata: "), nil
	default:
		b, err := json.Marshal(value)
		if err != nil {
			return "", err
		}
		return string(b), nil
	}
}

func hasSSEContentType(contentType string) bool {
	if contentType == "" {
		return false
	}

	mediaType := strings.ToLower(contentType)
	if idx := strings.IndexByte(mediaType, ';'); idx >= 0 {
		mediaType = strings.TrimSpace(mediaType[:idx])
	} else {
		mediaType = strings.TrimSpace(mediaType)
	}

	return mediaType == "text/event-stream"
}
