package middleware

import (
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/Pavan-Silva/go-zen"
)

// RateLimiterConfig holds configuration for rate limiting middleware.
type RateLimiterConfig struct {
	// Limit is the maximum number of requests per duration.
	Limit int
	// Duration is the time window for the rate limit.
	Duration time.Duration
	// KeyFunc extracts the key to rate limit by (e.g., IP address).
	// Default: client IP address.
	KeyFunc func(*http.Request) string
	// Skipper defines a function to skip middleware.
	Skipper zen.SkipFunc
}

// DefaultRateLimiterConfig returns a RateLimiterConfig with sensible defaults.
func DefaultRateLimiterConfig() RateLimiterConfig {
	return RateLimiterConfig{
		Limit:    100,
		Duration: time.Minute,
	}
}

// visitor tracks request count and reset time for a single client.
type visitor struct {
	count   int
	resetAt time.Time
}

// RateLimiter returns middleware that limits the number of requests per client.
// Uses an in-memory map with lazy cleanup on each request.
//
// Example:
//
//	r.Use(middleware.RateLimiter())
//
// Example with custom settings:
//
//	config := middleware.DefaultRateLimiterConfig()
//	config.Limit = 10
//	config.Duration = time.Second * 10
//	r.Use(middleware.RateLimiterWithConfig(config))
func RateLimiter() zen.MiddlewareFunc {
	return RateLimiterWithConfig(DefaultRateLimiterConfig())
}

// RateLimiterWithConfig returns rate limiter middleware with the given configuration.
func RateLimiterWithConfig(config RateLimiterConfig) zen.MiddlewareFunc {
	if config.Duration == 0 {
		config.Duration = time.Minute
	}
	if config.Limit == 0 {
		config.Limit = 100
	}
	if config.KeyFunc == nil {
		config.KeyFunc = func(r *http.Request) string {
			if ip := r.Header.Get("X-Forwarded-For"); ip != "" {
				return ip
			}
			if ip := r.Header.Get("X-Real-IP"); ip != "" {
				return ip
			}
			return r.RemoteAddr
		}
	}

	var (
		mu       sync.Mutex
		visitors = make(map[string]*visitor)
	)

	return func(c *zen.Context, next http.Handler) {
		if config.Skipper != nil && config.Skipper(c.Request) {
			next.ServeHTTP(c.Response, c.Request)
			return
		}

		key := config.KeyFunc(c.Request)
		now := time.Now()

		mu.Lock()

		// Lazy cleanup of expired entries
		for k, v := range visitors {
			if now.After(v.resetAt) {
				delete(visitors, k)
			}
		}

		v, exists := visitors[key]

		if !exists || now.After(v.resetAt) {
			visitors[key] = &visitor{
				count:   1,
				resetAt: now.Add(config.Duration),
			}
			limit := config.Limit
			remaining := limit - 1
			mu.Unlock()

			c.Response.Header().Set("X-RateLimit-Limit", fmt.Sprintf("%d", limit))
			c.Response.Header().Set("X-RateLimit-Remaining", fmt.Sprintf("%d", remaining))
			next.ServeHTTP(c.Response, c.Request)
			return
		}

		v.count++
		limit := config.Limit
		remaining := limit - v.count
		if remaining < 0 {
			remaining = 0
		}
		mu.Unlock()

		c.Response.Header().Set("X-RateLimit-Limit", fmt.Sprintf("%d", limit))
		c.Response.Header().Set("X-RateLimit-Remaining", fmt.Sprintf("%d", remaining))

		if v.count > limit {
			http.Error(c.Response, "Rate limit exceeded", http.StatusTooManyRequests)
			return
		}

		next.ServeHTTP(c.Response, c.Request)
	}
}
