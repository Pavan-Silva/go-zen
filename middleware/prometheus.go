package middleware

import (
	"net/http"
	"strconv"
	"time"

	"github.com/Pavan-Silva/go-zen"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var (
	httpRequests = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "zen",
			Subsystem: "http",
			Name:      "requests_total",
			Help:      "Total number of HTTP requests",
		},
		[]string{"method", "path", "status"},
	)

	httpDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: "zen",
			Subsystem: "http",
			Name:      "request_duration_seconds",
			Help:      "HTTP request duration in seconds",
			Buckets:   prometheus.DefBuckets,
		},
		[]string{"method", "path", "status"},
	)

	httpSize = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: "zen",
			Subsystem: "http",
			Name:      "response_size_bytes",
			Help:      "HTTP response size in bytes",
			Buckets:   prometheus.ExponentialBuckets(100, 10, 8),
		},
		[]string{"method", "path", "status"},
	)
)

// Prometheus returns middleware that collects Prometheus metrics for HTTP requests.
// It tracks request count, latency histogram, and response size.
//
// Example:
//
//	r.Use(middleware.Prometheus())
func Prometheus() zen.MiddlewareFunc {
	return func(c *zen.Context, next http.Handler) {
		start := time.Now()
		rw := &promResponseWriter{ResponseWriter: c.Response}

		next.ServeHTTP(rw, c.Request)

		duration := time.Since(start).Seconds()
		status := strconv.Itoa(rw.status)
		if rw.status == 0 {
			status = "200"
		}

		path := c.Request.URL.Path
		method := c.Request.Method

		httpRequests.WithLabelValues(method, path, status).Inc()
		httpDuration.WithLabelValues(method, path, status).Observe(duration)
		httpSize.WithLabelValues(method, path, status).Observe(float64(rw.written))
	}
}

// PrometheusHandler returns an http.Handler for the /metrics endpoint.
// Mount this on your router to expose Prometheus metrics.
//
// Example:
//
//	r.HandleRaw("GET /metrics", middleware.PrometheusHandler())
func PrometheusHandler() http.Handler {
	return promhttp.Handler()
}

type promResponseWriter struct {
	http.ResponseWriter
	status  int
	written int
}

func (w *promResponseWriter) WriteHeader(status int) {
	w.status = status
	w.ResponseWriter.WriteHeader(status)
}

func (w *promResponseWriter) Write(b []byte) (int, error) {
	if w.status == 0 {
		w.status = http.StatusOK
	}
	n, err := w.ResponseWriter.Write(b)
	w.written += n
	return n, err
}
