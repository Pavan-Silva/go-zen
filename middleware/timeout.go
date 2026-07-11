package middleware

import (
	"context"
	"time"

	"github.com/Pavan-Silva/go-zen"
	"github.com/Pavan-Silva/go-zen/logger"
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
// It sets a deadline on the request context. Handlers should pass the context
// to downstream operations (db queries, HTTP calls) to benefit from cancellation.
//
// If the deadline is exceeded, downstream operations that respect context
// cancellation will return early with a cancellation error.
func TimeoutWithConfig(config TimeoutConfig) zen.HandlerFunc {
	if config.Duration <= 0 {
		config.Duration = 30 * time.Second
	}

	return func(c *zen.Ctx) {
		if config.Skipper != nil && config.Skipper(c.Request) {
			c.Next()
			return
		}

		ctx, cancel := context.WithDeadline(c.Request.Context(), time.Now().Add(config.Duration))
		defer cancel()

		c.Request = c.Request.WithContext(ctx)

		logger.Debug("request timeout set",
			"method", c.Request.Method,
			"path", c.Request.URL.Path,
			"duration", config.Duration,
		)

		c.Next()
	}
}
