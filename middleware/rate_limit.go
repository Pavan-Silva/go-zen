package middleware

import (
	"math"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/Pavan-Silva/go-zen"
	"golang.org/x/time/rate"
)

// RateLimiterConfig holds configuration for rate limiting middleware.
type RateLimiterConfig struct {
	// Limit is the maximum number of requests per duration (also the burst size).
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

type perKeyLimiter struct {
	limiter  *rate.Limiter
	lastSeen time.Time
}

// RateLimiter returns middleware that limits the number of requests per client.
// Uses a token bucket (golang.org/x/time/rate) per client key.
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
func RateLimiter() zen.HandlerFunc {
	return RateLimiterWithConfig(DefaultRateLimiterConfig())
}

// RateLimiterWithConfig returns rate limiter middleware with the given configuration.
func RateLimiterWithConfig(config RateLimiterConfig) zen.HandlerFunc {
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

	r := rate.Limit(float64(config.Limit) / config.Duration.Seconds())
	burst := config.Limit

	var (
		mu       sync.Mutex
		limiters = make(map[string]*perKeyLimiter)
		stop     = make(chan struct{})
	)

	go func() {
		ticker := time.NewTicker(time.Minute)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				mu.Lock()
				now := time.Now()
				for k, v := range limiters {
					if now.After(v.lastSeen.Add(2 * config.Duration)) {
						delete(limiters, k)
					}
				}
				mu.Unlock()
			case <-stop:
				return
			}
		}
	}()

	return func(c *zen.Ctx) {
		if config.Skipper != nil && config.Skipper(c.Request) {
			c.Next()
			return
		}

		key := config.KeyFunc(c.Request)

		mu.Lock()
		pkl, exists := limiters[key]
		if !exists {
			pkl = &perKeyLimiter{
				limiter: rate.NewLimiter(r, burst),
			}
			limiters[key] = pkl
		}
		pkl.lastSeen = time.Now()
		mu.Unlock()

		if !pkl.limiter.Allow() {
			http.Error(c.Response, "Rate limit exceeded", http.StatusTooManyRequests)
			return
		}

		c.Response.Header().Set("X-RateLimit-Limit", strconv.Itoa(burst))
		c.Response.Header().Set("X-RateLimit-Remaining", strconv.Itoa(int(math.Ceil(pkl.limiter.Tokens()))))

		c.Next()
	}
}
