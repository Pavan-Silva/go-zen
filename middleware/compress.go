package middleware

import (
	"bytes"
	"compress/gzip"
	"io"
	"net/http"
	"strings"
	"sync"

	"github.com/Pavan-Silva/go-zen"
)

// Minimum body threshold length (1 KB) below which compression is skipped.
const minCompressionThreshold = 1024

type compressResponseWriter struct {
	http.ResponseWriter
	gz          *gzip.Writer
	bodyBuffer  *bytes.Buffer
	status      int
	level       int
	wroteHeader bool
	gzipPool    *sync.Pool
}

// Compress returns standard gzip compression middleware using default compression parameters.
func Compress() zen.HandlerFunc {
	return CompressWithLevel(gzip.DefaultCompression)
}

// CompressWithLevel returns compression middleware with a specific gzip level,
// utilizing isolated sync pools per level to prevent cross-contamination.
func CompressWithLevel(level int) zen.HandlerFunc {
	// Isolated local pools bound strictly to this function's lexical scope (closure)
	var gzipPool = sync.Pool{
		New: func() any {
			w, err := gzip.NewWriterLevel(io.Discard, level)
			if err != nil {
				panic("gzip optimization configuration fatal: " + err.Error())
			}
			return w
		},
	}

	var writerStructPool = sync.Pool{
		New: func() any {
			return &compressResponseWriter{
				bodyBuffer: bytes.NewBuffer(make([]byte, 0, minCompressionThreshold)),
			}
		},
	}

	return func(c *zen.Ctx) {
		// Fast Path 1: Validate client capabilities
		av := c.Request.Header["Accept-Encoding"]
		if len(av) == 0 || !strings.Contains(av[0], "gzip") {
			c.Next()
			return
		}

		// Fast Path 2: Skip if another middleware already compressed the response
		if _, exists := c.Response.Header()["Content-Encoding"]; exists {
			c.Next()
			return
		}

		cw := writerStructPool.Get().(*compressResponseWriter)
		cw.ResponseWriter = c.Response
		cw.status = http.StatusOK
		cw.level = level
		cw.wroteHeader = false
		cw.bodyBuffer.Reset()
		cw.gz = nil
		cw.gzipPool = &gzipPool

		c.Response = cw

		c.Next()

		if !cw.wroteHeader {
			cw.writeFinal(&gzipPool)
		} else if cw.gz != nil {
			_ = cw.gz.Close()
			gzipPool.Put(cw.gz)
		}

		cw.ResponseWriter = nil
		cw.gz = nil
		writerStructPool.Put(cw)
	}
}

func (w *compressResponseWriter) WriteHeader(status int) {
	if w.wroteHeader {
		return
	}
	w.status = status
}

func (w *compressResponseWriter) Write(b []byte) (int, error) {
	// Case 1: Stream is already initialized and gzipping directly to the network socket
	if w.gz != nil {
		return w.gz.Write(b)
	}

	// Case 2: Buffer is still accumulating bytes below the 1KB threshold
	if w.bodyBuffer.Len()+len(b) < minCompressionThreshold && !w.wroteHeader {
		return w.bodyBuffer.Write(b)
	}

	// Case 3: Payload crossed the threshold. Initialize gzip streaming immediately.
	// We pass a dummy pointer here since Write doesn't have access to the closure's pool.
	// Instead, we initialize a standalone writer if it escapes during an active write loop,
	// ensuring optimal stream continuity.
	w.initGzipStream()
	return w.gz.Write(b)
}

func (w *compressResponseWriter) initGzipStream() {
	w.wroteHeader = true
	w.Header().Set("Content-Encoding", "gzip")
	w.Header().Del("Content-Length")
	w.Header().Set("Vary", "Accept-Encoding")
	w.ResponseWriter.WriteHeader(w.status)

	w.gz = w.gzipPool.Get().(*gzip.Writer)
	w.gz.Reset(w.ResponseWriter)

	if w.bodyBuffer.Len() > 0 {
		_, _ = w.gz.Write(w.bodyBuffer.Bytes())
		w.bodyBuffer.Reset()
	}
}

// FIXED: Adjusted signature to accept `*sync.Pool` pointer reference to satisfy go vet analysis
func (w *compressResponseWriter) writeFinal(pool *sync.Pool) {
	w.wroteHeader = true

	// If the accumulated payload is small, skip gzip entirely to save CPU and bandwidth
	if w.bodyBuffer.Len() < minCompressionThreshold {
		w.ResponseWriter.WriteHeader(w.status)
		_, _ = w.ResponseWriter.Write(w.bodyBuffer.Bytes())
		return
	}

	// Payload is large enough to compress. Initialize gzip for the buffered data.
	w.Header().Set("Content-Encoding", "gzip")
	w.Header().Del("Content-Length")
	w.Header().Set("Vary", "Accept-Encoding")
	w.ResponseWriter.WriteHeader(w.status)

	gz := pool.Get().(*gzip.Writer)
	gz.Reset(w.ResponseWriter)

	_, _ = gz.Write(w.bodyBuffer.Bytes())
	_ = gz.Close()
	pool.Put(gz)
}

func (w *compressResponseWriter) Flush() {
	if w.gz != nil {
		_ = w.gz.Flush()
	}
	if f, ok := w.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}
