package zen

import (
	"bufio"
	"net"
	"net/http"
)

// delayedStatusWriter delays writing the status code until the first Write call.
// This allows JSON/XML serialization to fail without committing the status code,
// matching Echo's approach for handling encode errors.
type delayedStatusWriter struct {
	http.ResponseWriter
	status  int
	written bool
}

func (w *delayedStatusWriter) WriteHeader(code int) {
	w.status = code
}

func (w *delayedStatusWriter) Write(b []byte) (int, error) {
	if !w.written {
		w.ResponseWriter.WriteHeader(w.status)
		w.written = true
	}
	return w.ResponseWriter.Write(b)
}

func (w *delayedStatusWriter) Unwrap() http.ResponseWriter {
	return w.ResponseWriter
}

// Hijack implements http.Hijacker if the underlying ResponseWriter supports it.
func (w *delayedStatusWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	if h, ok := w.ResponseWriter.(http.Hijacker); ok {
		return h.Hijack()
	}
	return nil, nil, http.ErrNotSupported
}

// Push implements http.Pusher if the underlying ResponseWriter supports it.
func (w *delayedStatusWriter) Push(target string, opts *http.PushOptions) error {
	if p, ok := w.ResponseWriter.(http.Pusher); ok {
		return p.Push(target, opts)
	}
	return http.ErrNotSupported
}

// Flush implements http.Flusher if the underlying ResponseWriter supports it.
func (w *delayedStatusWriter) Flush() {
	if f, ok := w.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}
