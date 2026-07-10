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
	Limit    int                        // Maximum number of requests allowed within the Duration.
	Duration time.Duration              // Time window for the rate limit.
	KeyFunc  func(*http.Request) string // Function to extract a unique key per client (default: client IP).
	Skipper  zen.SkipFunc               // Optional function to skip rate limiting for certain requests.
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

const shardCount = 64

type limiterShard struct {
	mu       sync.RWMutex
	limiters map[string]*perKeyLimiter
}

// RateLimiter returns middleware that limits the number of requests per client.
func RateLimiter() zen.HandlerFunc {
	return RateLimiterWithConfig(DefaultRateLimiterConfig())
}

// RateLimiterWithConfig returns rate limiting middleware.
func RateLimiterWithConfig(config RateLimiterConfig) zen.HandlerFunc {
	if config.Duration == 0 {
		config.Duration = time.Minute
	}
	if config.Limit == 0 {
		config.Limit = 100
	}
	if config.KeyFunc == nil {
		config.KeyFunc = func(r *http.Request) string {
			// Extract client IP efficiently
			if ip := r.Header["X-Forwarded-For"]; len(ip) > 0 {
				return ip[0]
			}
			if ip := r.Header["X-Real-Ip"]; len(ip) > 0 {
				return ip[0]
			}
			return r.RemoteAddr
		}
	}

	r := rate.Limit(float64(config.Limit) / config.Duration.Seconds())
	burst := config.Limit

	// Pre-compute burst value for headers.
	burstStr := strconv.Itoa(burst)

	// Initialize shards.
	shards := make([]*limiterShard, shardCount)
	for i := range shardCount {
		shards[i] = &limiterShard{
			limiters: make(map[string]*perKeyLimiter),
		}
	}

	// Periodic cleanup of stale limiters.
	// Runs for the process lifetime - rate limiter middleware is registered once at startup.
	go func() {
		ticker := time.NewTicker(time.Minute)
		for range ticker.C {
			now := time.Now()
			for i := range shardCount {
				shard := shards[i]
				shard.mu.Lock()
				for k, v := range shard.limiters {
					if now.After(v.lastSeen.Add(2 * config.Duration)) {
						delete(shard.limiters, k)
					}
				}
				shard.mu.Unlock()
			}
		}
	}()

	return func(c *zen.Ctx) {
		if config.Skipper != nil && config.Skipper(c.Request) {
			c.Next()
			return
		}

		key := config.KeyFunc(c.Request)

		// Hash key to select shard.
		var hash uint32 = 2166136261
		for i := 0; i < len(key); i++ {
			hash ^= uint32(key[i])
			hash *= 16777619
		}
		shardIdx := hash % shardCount
		shard := shards[shardIdx]

		// Try read-lock first.
		shard.mu.RLock()
		pkl, exists := shard.limiters[key]
		shard.mu.RUnlock()

		if !exists {
			// Acquire write-lock to create new limiter.
			shard.mu.Lock()
			// Double-check after lock.
			pkl, exists = shard.limiters[key]
			if !exists {
				pkl = &perKeyLimiter{
					limiter: rate.NewLimiter(r, burst),
				}
				shard.limiters[key] = pkl
			}
			shard.mu.Unlock()
		}

		shard.mu.Lock()
		pkl.lastSeen = time.Now()
		shard.mu.Unlock()

		if !pkl.limiter.Allow() {
			c.Response.WriteHeader(http.StatusTooManyRequests)
			_, _ = c.Response.Write(zen.StringToBytes("Rate limit exceeded"))
			return
		}

		// Set rate limit response headers.
		headers := c.Response.Header()
		headers["X-Ratelimit-Limit"] = []string{burstStr}

		remainingTokens := int(math.Ceil(pkl.limiter.Tokens()))
		headers["X-Ratelimit-Remaining"] = []string{strconv.Itoa(remainingTokens)}

		c.Next()
	}
}
