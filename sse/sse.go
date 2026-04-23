package sse

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/Pavan-Silva/go-zen"
	"github.com/Pavan-Silva/go-zen/auth"
)

const contentType = "text/event-stream;charset=utf-8"

var ErrFlusherUnsupported = errors.New("sse: SSE requires http.Flusher")

// Send writes a Server-Sent Event to the client.
// It sets required SSE headers and flushes the response.
func Send(c *zen.Context, event string, data any) error {
	flusher, ok := c.Response.(http.Flusher)
	if !ok {
		return ErrFlusherUnsupported
	}

	headers := c.Response.Header()
	// Set SSE headers only if not already set (first call per response)
	if headers.Get("Content-Type") != contentType {
		headers.Set("Content-Type", contentType)
		headers.Set("Cache-Control", "no-cache")
		headers.Set("Connection", "keep-alive")
		headers.Set("X-Accel-Buffering", "no")
	}

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

// Handle registers an SSE route on the given Router.
// The handler receives the zen Context and returns any error it wants
// to propagate as a 500 response.
func Handle(r *zen.Router, pattern string, handler func(*zen.Context) error) {
	r.Handle(pattern, func(c *zen.Context) {
		if err := handler(c); err != nil {
			c.Error(http.StatusInternalServerError, err.Error())
		}
	})
}

// HandleWithAuth registers an authenticated SSE route.
// The authenticator is called before the handler, and the authenticated User
// is stored in the Context under "user".
func HandleWithAuth(r *zen.Router, pattern string, authenticator auth.Authenticator, handler func(*zen.Context) error) {
	r.Handle(pattern, func(c *zen.Context) {
		user, err := auth.AuthenticateSSE(authenticator, c.Request)
		if err != nil {
			c.Error(http.StatusUnauthorized, http.StatusText(http.StatusUnauthorized))
			return
		}

		c.Set("user", user)
		if err := handler(c); err != nil {
			c.Error(http.StatusInternalServerError, err.Error())
		}
	})
}
