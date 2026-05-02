package middleware

import (
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/Pavan-Silva/go-zen"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// PrometheusConfig holds configuration for Prometheus middleware.
type PrometheusConfig struct {
	// Namespace is the prefix for all metrics (e.g., "myapp").
	Namespace string
	// Subsystem is the subsystem for metrics (e.g., "http").
	Subsystem string
	// Skipper defines a function to skip middleware.
	Skipper zen.SkipFunc
}

// DefaultPrometheusConfig returns a PrometheusConfig with sensible defaults.
func DefaultPrometheusConfig() PrometheusConfig {
	return PrometheusConfig{
		Namespace: "zen",
		Subsystem: "http",
	}
}

var (
	// Metrics are initialized once per config
	metricsOnce sync.Once
	httpRequests *prometheus.CounterVec
	httpDuration *prometheus.HistogramVec
	httpSize     *prometheus.HistogramVec
)

// Prometheus returns middleware that collects Prometheus metrics for HTTP requests.
// It tracks request count, latency histogram, and response size.
//
// Example:
//
//	r.Use(middleware.Prometheus())
//
// Example with custom config:
//
//	config := middleware.DefaultPrometheusConfig()
//	config.Namespace = "myapp"
//	r.Use(middleware.PrometheusWithConfig(config))
func Prometheus() zen.MiddlewareFunc {
	return PrometheusWithConfig(DefaultPrometheusConfig())
}

// PrometheusWithConfig returns Prometheus middleware with the given configuration.
func PrometheusWithConfig(config PrometheusConfig) zen.MiddlewareFunc {
	if config.Namespace == "" {
		config.Namespace = "zen"
	}
	if config.Subsystem == "" {
		config.Subsystem = "http"
	}

	metricsOnce.Do(func() {
		httpRequests = promauto.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: config.Namespace,
				Subsystem: config.Subsystem,
				Name:      "requests_total",
				Help:      "Total number of HTTP requests",
			},
			[]string{"method", "path", "status"},
		)

		httpDuration = promauto.NewHistogramVec(
			prometheus.HistogramOpts{
				Namespace: config.Namespace,
				Subsystem: config.Subsystem,
				Name:      "request_duration_seconds",
				Help:      "HTTP request duration in seconds",
				Buckets:   prometheus.DefBuckets,
			},
			[]string{"method", "path", "status"},
		)

		httpSize = promauto.NewHistogramVec(
			prometheus.HistogramOpts{
				Namespace: config.Namespace,
				Subsystem: config.Subsystem,
				Name:      "response_size_bytes",
				Help:      "HTTP response size in bytes",
				Buckets:   prometheus.ExponentialBuckets(100, 10, 8),
			},
			[]string{"method", "path", "status"},
		)
	})

	return func(c *zen.Context, next http.Handler) {
		if config.Skipper != nil && config.Skipper(c.Request) {
			next.ServeHTTP(c.Response, c.Request)
			return
		}

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

// init registers the metrics endpoint helper
func init() {
	// Metrics are lazy-initialized on first use
}
