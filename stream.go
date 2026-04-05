package zen

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"golang.org/x/net/websocket"
)

const (
	sseContentType = "text/event-stream;charset=utf-8"
)

var errSSEUnsupported = errors.New("zen: SSE requires http.Flusher")

// SSEvent sends a Server-Sent Event to the client.
// It automatically sets the required SSE headers and flushes the stream.
// The data payload is JSON-encoded for non-string values.
func (c *Context) SSEvent(event string, data any) error {
	flusher, ok := c.Response.(http.Flusher)
	if !ok {
		return errSSEUnsupported
	}

	headers := c.Response.Header()
	headers.Set("Content-Type", sseContentType)
	headers.Set("Cache-Control", "no-cache")
	headers.Set("Connection", "keep-alive")
	headers.Set("X-Accel-Buffering", "no")

	payload, err := encodeSSEData(data)
	if err != nil {
		return err
	}

	if event != "" {
		if _, err := fmt.Fprintf(c.Response, "event: %s\n", event); err != nil {
			return err
		}
	}

	if _, err := fmt.Fprintf(c.Response, "data: %s\n\n", payload); err != nil {
		return err
	}

	flusher.Flush()
	return nil
}

func encodeSSEData(data any) (string, error) {
	switch value := data.(type) {
	case string:
		return strings.ReplaceAll(value, "\n", "\ndata: "), nil
	case []byte:
		return strings.ReplaceAll(string(value), "\n", "\ndata: "), nil
	default:
		b, err := json.Marshal(value)
		if err != nil {
			return "", err
		}
		return strings.ReplaceAll(string(b), "\n", "\ndata: "), nil
	}
}

// WebSocketConn wraps a websocket connection and provides JSON-friendly helpers.
type WebSocketConn struct {
	conn *websocket.Conn
}

// ReadJSON reads the next JSON message from the WebSocket.
func (ws *WebSocketConn) ReadJSON(dest any) error {
	return websocket.JSON.Receive(ws.conn, dest)
}

// WriteJSON writes a JSON-encoded message to the WebSocket.
func (ws *WebSocketConn) WriteJSON(message any) error {
	return websocket.JSON.Send(ws.conn, message)
}

// Close closes the WebSocket connection.
func (ws *WebSocketConn) Close() error {
	return ws.conn.Close()
}

// HandleWebSocket registers a WebSocket route with the given pattern.
// The provided handler receives the request Context and a WebSocketConn.
func (s *Router) HandleWebSocket(pattern string, handler func(*Context, *WebSocketConn)) {
	wsHandler := websocket.Handler(func(conn *websocket.Conn) {
		c := FromRequest(conn.Request())
		if c == nil {
			_ = conn.Close()
			return
		}
		handler(c, &WebSocketConn{conn: conn})
	})

	s.mux.Handle(pattern, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c, r := newContext(w, r)
		wsHandler.ServeHTTP(w, r)
		releaseContext(c)
	}))
}
