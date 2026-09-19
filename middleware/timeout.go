package middleware

import (
	"context"
	"net/http"
	"sync"
	"time"

	"github.com/Pavan-Silva/go-zen"
	"github.com/Pavan-Silva/go-zen/internal/log"
)

// TimeoutConfig holds configuration for the request timeout middleware.
type TimeoutConfig struct {
	// Duration is the maximum time a request can take.
	Duration time.Duration

	// Skipper optionally skips certain requests.
	Skipper zen.SkipFunc
}

// DefaultTimeoutConfig returns a TimeoutConfig with sensible defaults.
func DefaultTimeoutConfig() TimeoutConfig {
	return TimeoutConfig{
		Duration: 30 * time.Second,
	}
}

// Timeout returns middleware that cancels requests exceeding the configured duration.
func Timeout(duration time.Duration) zen.HandlerFunc {
	return TimeoutWithConfig(TimeoutConfig{Duration: duration})
}

// TimeoutWithConfig returns timeout middleware with the given config.
//
// It sets a deadline on the request context so context-aware handlers (db
// queries, HTTP calls) cancel early. Once the deadline passes, the response
// is marked as timed out: the next write attempt emits a 504 Gateway Timeout
// and discards the handler's payload, and any subsequent writes are dropped.
//
// Because the handler runs in the request goroutine and the 504 is written by
// the handler's own write path (or in teardown after it returns), the response
// writer is never accessed concurrently, and the pooled Ctx can never be
// reused while the handler is still active.
//
// A handler that blocks forever while ignoring the request context cannot be
// interrupted; it must return (e.g. by observing ctx.Done()) for the timeout
// to take effect.
func TimeoutWithConfig(config TimeoutConfig) zen.HandlerFunc {
	if config.Duration <= 0 {
		config.Duration = 30 * time.Second
	}

	return func(c *zen.Ctx) {
		if skipIfSkipped(c, config.Skipper) {
			return
		}

		deadline := time.Now().Add(config.Duration)
		ctx, cancel := context.WithDeadline(c.Request.Context(), deadline)
		defer cancel()
		c.Request = c.Request.WithContext(ctx)

		tw := &timeoutResponseWriter{ResponseWriter: c.Response}
		c.Response = tw

		done := make(chan struct{})
		defer close(done)
		timer := time.AfterFunc(config.Duration, func() {
			select {
			case <-done:
			default:
				tw.timeout()
			}
		})
		defer timer.Stop()

		log.Debug("request timeout set, method=%s path=%s duration=%s",
			c.Request.Method, c.Request.URL.Path, config.Duration)

		c.Next()

		// The handler returned after the deadline without committing a
		// response: emit the 504 from the handler goroutine.
		if time.Now().After(deadline) && tw.status == 0 {
			tw.commitHeader(http.StatusGatewayTimeout)
		}
	}
}

// timeoutResponseWriter serializes writes so the timeout flag flips atomically.
// Only the handler goroutine ever touches the underlying ResponseWriter, so no
// concurrent access is introduced.
type timeoutResponseWriter struct {
	http.ResponseWriter
	mu       sync.Mutex
	timedOut bool
	status   int
}

// timeout marks the response as timed out, discarding subsequent writes.
func (tw *timeoutResponseWriter) timeout() {
	tw.mu.Lock()
	tw.timedOut = true
	tw.mu.Unlock()
}

// rejectIfTimedOut reports whether the response timed out and, when it did,
// emits a 504 Gateway Timeout if no status was committed yet. Writes are
// discarded when it returns true.
func (tw *timeoutResponseWriter) rejectIfTimedOut() bool {
	tw.mu.Lock()
	defer tw.mu.Unlock()
	if !tw.timedOut {
		return false
	}
	if tw.status == 0 {
		tw.status = http.StatusGatewayTimeout
		tw.ResponseWriter.WriteHeader(http.StatusGatewayTimeout)
	}
	return true
}

// commitHeader writes a status line when none was written yet.
func (tw *timeoutResponseWriter) commitHeader(status int) {
	tw.mu.Lock()
	defer tw.mu.Unlock()
	if tw.status != 0 {
		return
	}
	tw.status = status
	tw.ResponseWriter.WriteHeader(status)
}

func (tw *timeoutResponseWriter) WriteHeader(status int) {
	if tw.rejectIfTimedOut() {
		return
	}
	tw.commitHeader(status)
}

func (tw *timeoutResponseWriter) Write(b []byte) (int, error) {
	if tw.rejectIfTimedOut() {
		return len(b), nil
	}
	tw.commitHeader(http.StatusOK) // an implicit 200 on the first write
	return tw.ResponseWriter.Write(b)
}

// Flush implements http.Flusher.
func (tw *timeoutResponseWriter) Flush() {
	if tw.rejectIfTimedOut() {
		return
	}
	tw.commitHeader(http.StatusOK)
	if f, ok := tw.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

// Unwrap exposes the underlying http.ResponseWriter for http.ResponseController.
func (tw *timeoutResponseWriter) Unwrap() http.ResponseWriter {
	return tw.ResponseWriter
}
