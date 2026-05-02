package middleware

import (
	"net/http"
	"time"

	"github.com/Pavan-Silva/go-zen"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"
)

// OTelConfig holds configuration for OpenTelemetry middleware.
type OTelConfig struct {
	// TracerProvider is the OpenTelemetry TracerProvider to use.
	// If nil, otel.GetTracerProvider() is used.
	TracerProvider trace.TracerProvider
	// Propagator is used to extract and inject trace context.
	// If nil, otel.GetTextMapPropagator() is used.
	Propagator propagation.TextMapPropagator
	// ServiceName is the name of the service for tracing.
	ServiceName string
	// Skipper defines a function to skip middleware.
	Skipper zen.SkipFunc
}

// DefaultOTelConfig returns an OTelConfig with sensible defaults.
func DefaultOTelConfig() OTelConfig {
	return OTelConfig{
		ServiceName: "go-zen-service",
	}
}

// OpenTelemetry returns middleware that adds distributed tracing via OpenTelemetry.
// It creates a span for each request and sets standard span attributes.
//
// Example:
//
//	r.Use(middleware.OpenTelemetry())
//
// Example with custom config:
//
//	config := middleware.DefaultOTelConfig()
//	config.ServiceName = "my-api"
//	r.Use(middleware.OpenTelemetryWithConfig(config))
func OpenTelemetry() zen.MiddlewareFunc {
	return OpenTelemetryWithConfig(DefaultOTelConfig())
}

// OpenTelemetryWithConfig returns OpenTelemetry middleware with the given configuration.
func OpenTelemetryWithConfig(config OTelConfig) zen.MiddlewareFunc {
	if config.ServiceName == "" {
		config.ServiceName = "go-zen-service"
	}

	tracer := otel.GetTracerProvider().Tracer(config.ServiceName)
	if config.TracerProvider != nil {
		tracer = config.TracerProvider.Tracer(config.ServiceName)
	}

	propagator := otel.GetTextMapPropagator()
	if config.Propagator != nil {
		propagator = config.Propagator
	}

	return func(c *zen.Context, next http.Handler) {
		if config.Skipper != nil && config.Skipper(c.Request) {
			next.ServeHTTP(c.Response, c.Request)
			return
		}

		ctx := propagator.Extract(c.Request.Context(), propagation.HeaderCarrier(c.Request.Header))

		ctx, span := tracer.Start(ctx, c.Request.Method+" "+c.Request.URL.Path,
			trace.WithAttributes(
				attribute.String("http.method", c.Request.Method),
				attribute.String("http.url", c.Request.URL.String()),
				attribute.String("http.route", c.Request.URL.Path),
				attribute.String("http.user_agent", c.Request.UserAgent()),
				attribute.String("http.client_ip", clientIP(c.Request)),
			),
		)
		defer span.End()

		// Wrap request with trace context
		c.Request = c.Request.WithContext(ctx)

		// Use a response writer wrapper to capture status code
		rw := &otelResponseWriter{ResponseWriter: c.Response}
		start := time.Now()

		next.ServeHTTP(rw, c.Request)

		// Record response attributes
		span.SetAttributes(
			attribute.Int("http.status_code", rw.status),
			attribute.Float64("http.duration_ms", time.Since(start).Seconds()*1000),
		)

		if rw.status >= 400 {
			span.SetAttributes(attribute.Bool("error", true))
		}
	}
}

type otelResponseWriter struct {
	http.ResponseWriter
	status int
}

func (w *otelResponseWriter) WriteHeader(status int) {
	w.status = status
	w.ResponseWriter.WriteHeader(status)
}

func (w *otelResponseWriter) Write(b []byte) (int, error) {
	if w.status == 0 {
		w.status = http.StatusOK
	}
	return w.ResponseWriter.Write(b)
}
