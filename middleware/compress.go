package middleware

import (
	"compress/gzip"
	"io"
	"net/http"
	"strings"
	"sync"

	"github.com/Pavan-Silva/go-zen"
)

var gzipWriterPool = sync.Pool{
	New: func() any {
		w, _ := gzip.NewWriterLevel(io.Discard, gzip.DefaultCompression)
		return w
	},
}

// CompressConfig holds configuration for compression middleware.
type CompressConfig struct {
	// Level is the gzip compression level (gzip.DefaultCompression, gzip.BestSpeed, etc.)
	Level int
	// Skipper defines a function to skip compression for certain requests.
	Skipper zen.SkipFunc
}

// DefaultCompressConfig returns a CompressConfig with sensible defaults.
func DefaultCompressConfig() CompressConfig {
	return CompressConfig{
		Level: gzip.DefaultCompression,
	}
}

// compressResponseWriter wraps http.ResponseWriter to capture written data and compress it.
type compressResponseWriter struct {
	http.ResponseWriter
	gz     *gzip.Writer
	status int
}

func (w *compressResponseWriter) WriteHeader(status int) {
	w.status = status
	w.Header().Set("Content-Encoding", "gzip")
	w.Header().Del("Content-Length")
	w.ResponseWriter.WriteHeader(status)
}

func (w *compressResponseWriter) Write(b []byte) (int, error) {
	if w.status == 0 {
		w.WriteHeader(http.StatusOK)
	}
	return w.gz.Write(b)
}

func (w *compressResponseWriter) Flush() {
	w.gz.Flush()
	if f, ok := w.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

// Compress returns gzip compression middleware.
// It compresses responses for clients that accept gzip encoding.
// Uses a sync.Pool for gzip.Writers to minimize allocations.
//
// Example:
//
//	r.Use(middleware.Compress())
//
// Example with custom level:
//
//	r.Use(middleware.CompressWithLevel(gzip.BestSpeed))
func Compress() zen.MiddlewareFunc {
	return CompressWithConfig(DefaultCompressConfig())
}

// CompressWithLevel returns compression middleware with a specific gzip level.
func CompressWithLevel(level int) zen.MiddlewareFunc {
	return CompressWithConfig(CompressConfig{Level: level})
}

// CompressWithConfig returns compression middleware with the given configuration.
//
// Example:
//
//	config := middleware.DefaultCompressConfig()
//	config.Level = gzip.BestSpeed
//	r.Use(middleware.CompressWithConfig(config))
func CompressWithConfig(config CompressConfig) zen.MiddlewareFunc {
	if config.Level == 0 {
		config.Level = gzip.DefaultCompression
	}
	return func(c *zen.Context, next http.Handler) {
		if config.Skipper != nil && config.Skipper(c.Request) {
			next.ServeHTTP(c.Response, c.Request)
			return
		}

		// Check if client accepts gzip
		accept := c.Request.Header.Get("Accept-Encoding")
		if !strings.Contains(accept, "gzip") {
			next.ServeHTTP(c.Response, c.Request)
			return
		}

		// Skip if response already has Content-Encoding set
		if c.Response.Header().Get("Content-Encoding") != "" {
			next.ServeHTTP(c.Response, c.Request)
			return
		}

		// Get gzip writer from pool
		gz := gzipWriterPool.Get().(*gzip.Writer)
		gz.Reset(c.Response)
		defer func() {
			gz.Close()
			gzipWriterPool.Put(gz)
		}()

		cw := &compressResponseWriter{ResponseWriter: c.Response, gz: gz}
		next.ServeHTTP(cw, c.Request)
	}
}
