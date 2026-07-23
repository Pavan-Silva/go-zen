package middleware

import (
	"bufio"
	"net"
	"net/http"
	"os"
	"strconv"
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
	spaces       = []byte("                    ") // 20 spaces for padding
)

var rwPool = sync.Pool{
	New: func() any { return &responseWriter{} },
}

var logBufPool = sync.Pool{
	New: func() any {
		b := make([]byte, 0, 256)
		return &b
	},
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

	bufPtr := logBufPool.Get().(*[]byte)
	buf := (*bufPtr)[:0]

	buf = append(buf, "[ZEN] "...)
	buf = start.AppendFormat(buf, "2006/01/02 - 15:04:05")
	buf = append(buf, " | "...)
	buf = appendStatusColor(buf, rw.status)
	buf = rightPad(buf, strconv.Itoa(rw.status), 5)
	buf = append(buf, colorReset...)
	buf = append(buf, " | "...)

	buf = rightPad(buf, formatLatency(duration), 9)
	buf = append(buf, " | "...)

	buf = leftPad(buf, c.ClientIP(), 15)
	buf = append(buf, " | "...)
	size := strconv.FormatInt(c.Request.ContentLength, 10) + "->" + strconv.FormatInt(int64(rw.written), 10) + "B"
	buf = rightPad(buf, size, 13)
	buf = append(buf, " | "...)

	buf = appendMethodColor(buf, method)
	buf = leftPad(buf, method, 7)
	buf = append(buf, colorReset...)
	buf = append(buf, " "...)

	buf = append(buf, path...)
	buf = append(buf, '\n')

	_, _ = os.Stdout.Write(buf)

	*bufPtr = buf
	logBufPool.Put(bufPtr)

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

func rightPad(buf []byte, s string, width int) []byte {
	n := width - len(s)
	if n > 0 {
		if n > len(spaces) {
			n = len(spaces)
		}
		buf = append(buf, spaces[:n]...)
	}
	buf = append(buf, s...)
	return buf
}

func leftPad(buf []byte, s string, width int) []byte {
	buf = append(buf, s...)
	n := width - len(s)
	if n > 0 {
		if n > len(spaces) {
			n = len(spaces)
		}
		buf = append(buf, spaces[:n]...)
	}
	return buf
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

func formatLatency(d time.Duration) string {
	switch {
	case d > time.Second:
		return strconv.FormatFloat(float64(d)/float64(time.Second), 'f', 2, 64) + "s"
	case d > time.Millisecond:
		return strconv.FormatFloat(float64(d)/float64(time.Millisecond), 'f', 2, 64) + "ms"
	case d > time.Microsecond:
		return strconv.FormatFloat(float64(d)/float64(time.Microsecond), 'f', 1, 64) + "us"
	default:
		return strconv.FormatInt(int64(d), 10) + "ns"
	}
}
