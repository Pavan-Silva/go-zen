package middleware

import (
	"compress/gzip"
	"io"
	"net/http"
	"strings"
	"sync"

	"github.com/Pavan-Silva/go-zen"
	"github.com/Pavan-Silva/go-zen/logger"
)

var gzipWriterPool = sync.Pool{
	New: func() any {
		w, err := gzip.NewWriterLevel(io.Discard, gzip.DefaultCompression)
		if err != nil {
			panic("gzip.NewWriterLevel: " + err.Error())
		}
		return w
	},
}

// Compress returns gzip compression middleware using default compression.
// It compresses responses for clients that accept gzip encoding.
//
// Example:
//
//	r.Use(middleware.Compress())
func Compress() zen.MiddlewareFunc {
	return compressWithLevel(gzip.DefaultCompression, nil)
}

// CompressWithLevel returns compression middleware with a specific gzip level.
//
// Example:
//
//	r.Use(middleware.CompressWithLevel(gzip.BestSpeed))
func CompressWithLevel(level int) zen.MiddlewareFunc {
	return compressWithLevel(level, nil)
}

// CompressWithSkipper returns compression middleware with a skipper function.
func CompressWithSkipper(skipper zen.SkipFunc) zen.MiddlewareFunc {
	return compressWithLevel(gzip.DefaultCompression, skipper)
}

func compressWithLevel(level int, skipper zen.SkipFunc) zen.MiddlewareFunc {
	return func(c *zen.Context, next http.Handler) {
		if skipper != nil && skipper(c.Request) {
			next.ServeHTTP(c.Response, c.Request)
			return
		}

		// Check if client accepts gzip
		if !strings.Contains(c.Request.Header.Get("Accept-Encoding"), "gzip") {
			next.ServeHTTP(c.Response, c.Request)
			return
		}

		// Skip if response already has Content-Encoding set
		if c.Response.Header().Get("Content-Encoding") != "" {
			next.ServeHTTP(c.Response, c.Request)
			return
		}

		gz := gzipWriterPool.Get().(*gzip.Writer)
		gz.Reset(c.Response)
		defer func() {
			if err := gz.Close(); err != nil {
				logger.Warn("gzip: close failed: %v", err)
			}
			gzipWriterPool.Put(gz)
		}()

		cw := &compressResponseWriter{ResponseWriter: c.Response, gz: gz}
		next.ServeHTTP(cw, c.Request)
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
	if err := w.gz.Flush(); err != nil {
		logger.Warn("gzip: flush failed: %v", err)
	}
	if f, ok := w.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}
