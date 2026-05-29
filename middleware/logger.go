package middleware

import (
	"bufio"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/Pavan-Silva/go-zen"
)

// Pre-allocated ANSI color codes as byte arrays
var (
	colorReset   = []byte("\033[0m")
	colorRed     = []byte("\033[31m")
	colorGreen   = []byte("\033[32m")
	colorYellow  = []byte("\033[33m")
	colorBlue    = []byte("\033[34m")
	colorMagenta = []byte("\033[35m")
	colorCyan    = []byte("\033[36m")
)

var rwPool = sync.Pool{
	New: func() any { return &responseWriter{} },
}

// Logger logs HTTP requests with method, path, status code, and response time.
func Logger(c *zen.Ctx) {
	start := time.Now()

	rw := rwPool.Get().(*responseWriter)
	rw.ResponseWriter = c.Response
	rw.status = http.StatusOK
	rw.written = 0

	c.Response = rw

	c.Next()

	duration := time.Since(start)
	method := c.Request.Method
	path := c.Request.URL.Path

	buf := make([]byte, 0, 160)

	buf = append(buf, "[ZEN] "...)
	buf = appendStatusColor(buf, rw.status)
	buf = strconv.AppendInt(buf, int64(rw.status), 10)
	buf = append(buf, colorReset...)
	buf = append(buf, " | "...)

	buf = appendLatency(buf, duration)
	buf = append(buf, " | "...)

	buf = append(buf, clientIP(c.Request)...)
	buf = append(buf, " | "...)
	buf = strconv.AppendInt(buf, c.Request.ContentLength, 10)
	buf = append(buf, "->"...)
	buf = strconv.AppendInt(buf, int64(rw.written), 10)
	buf = append(buf, "B | "...)

	buf = appendMethodColor(buf, method)
	buf = append(buf, method...)
	buf = append(buf, colorReset...)
	buf = append(buf, " "...)

	buf = append(buf, path...)
	buf = append(buf, '\n')

	_, _ = os.Stdout.Write(buf)

	rw.ResponseWriter = nil
	rwPool.Put(rw)
}

// responseWriter wraps http.ResponseWriter to capture status codes and body sizes.
type responseWriter struct {
	http.ResponseWriter
	status  int
	written int
}

func (rw *responseWriter) WriteHeader(status int) {
	rw.status = status
	rw.ResponseWriter.WriteHeader(status)
}

func (rw *responseWriter) Write(b []byte) (int, error) {
	n, err := rw.ResponseWriter.Write(b)
	rw.written += n
	return n, err
}

// Flush implements http.Flusher.
func (rw *responseWriter) Flush() {
	if flusher, ok := rw.ResponseWriter.(http.Flusher); ok {
		flusher.Flush()
	}
}

// Hijack implements http.Hijacker.
func (rw *responseWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	if hijacker, ok := rw.ResponseWriter.(http.Hijacker); ok {
		return hijacker.Hijack()
	}
	return nil, nil, http.ErrNotSupported
}

// clientIP extracts the client IP address efficiently.
func clientIP(r *http.Request) string {
	if fwd := r.Header["X-Forwarded-For"]; len(fwd) > 0 {
		if before, _, ok := strings.Cut(fwd[0], ","); ok {
			return strings.TrimSpace(before)
		}
		return strings.TrimSpace(fwd[0])
	}
	if realIP := r.Header["X-Real-Ip"]; len(realIP) > 0 {
		return strings.TrimSpace(realIP[0])
	}
	return r.RemoteAddr
}

func appendStatusColor(buf []byte, status int) []byte {
	switch {
	case status >= 200 && status < 300:
		return append(buf, colorGreen...)
	case status >= 300 && status < 400:
		return append(buf, colorCyan...)
	case status >= 400 && status < 500:
		return append(buf, colorYellow...)
	default:
		return append(buf, colorRed...)
	}
}

func appendMethodColor(buf []byte, method string) []byte {
	switch method {
	case "GET":
		return append(buf, colorBlue...)
	case "POST":
		return append(buf, colorCyan...)
	case "PUT":
		return append(buf, colorYellow...)
	case "DELETE":
		return append(buf, colorRed...)
	default:
		return append(buf, colorMagenta...)
	}
}

func appendLatency(buf []byte, d time.Duration) []byte {
	switch {
	case d > time.Second:
		buf = strconv.AppendFloat(buf, float64(d)/float64(time.Second), 'f', 2, 64)
		return append(buf, 's')
	case d > time.Millisecond:
		buf = strconv.AppendFloat(buf, float64(d)/float64(time.Millisecond), 'f', 1, 64)
		return append(buf, "ms"...)
	case d > time.Microsecond:
		buf = strconv.AppendFloat(buf, float64(d)/float64(time.Microsecond), 'f', 1, 64)
		return append(buf, "µs"...)
	default:
		buf = strconv.AppendInt(buf, int64(d), 10)
		return append(buf, "ns"...)
	}
}
