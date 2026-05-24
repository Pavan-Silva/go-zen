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
	Limit    int
	Duration time.Duration
	KeyFunc  func(*http.Request) string
	Skipper  zen.SkipFunc
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

// Fixed-size configuration array to shard localized mutex structures.
const shardCount = 64

type limiterShard struct {
	mu       sync.RWMutex
	limiters map[string]*perKeyLimiter
}

// RateLimiter returns middleware that limits the number of requests per client.
func RateLimiter() zen.HandlerFunc {
	return RateLimiterWithConfig(DefaultRateLimiterConfig())
}

// RateLimiterWithConfig returns rate limiter middleware with a sharded lock layout.
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

	// Pre-convert static burst metric to eliminate runtime string allocation overhead
	burstStr := strconv.Itoa(burst)

	// Initialize individual lock memory groups to distribute parallel traffic
	shards := make([]*limiterShard, shardCount)
	for i := range shardCount {
		shards[i] = &limiterShard{
			limiters: make(map[string]*perKeyLimiter),
		}
	}

	// Dynamic sliding cleanup worker loop
	go func() {
		ticker := time.NewTicker(time.Minute)
		defer ticker.Stop()
		for range ticker.C {
			now := time.Now()
			// Clean up each map shard independently to avoid blocking global traffic
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

		// Low-overhead string FNV-1a hash algorithm to select the target map shard index
		var hash uint32 = 2166136261
		for i := 0; i < len(key); i++ {
			hash ^= uint32(key[i])
			hash *= 16777619
		}
		shardIdx := hash % shardCount
		shard := shards[shardIdx]

		// Fast Path: Try a read-lock first to see if the client already has a limiter
		shard.mu.RLock()
		pkl, exists := shard.limiters[key]
		shard.mu.RUnlock()

		if !exists {
			// Upgrade to a write-lock to add the new client limiter
			shard.mu.Lock()
			// Double-check existence to prevent race conditions during the lock upgrade
			pkl, exists = shard.limiters[key]
			if !exists {
				pkl = &perKeyLimiter{
					limiter: rate.NewLimiter(r, burst),
				}
				shard.limiters[key] = pkl
			}
			shard.mu.Unlock()
		}

		// Update tracking metrics using a localized write-lock
		shard.mu.Lock()
		pkl.lastSeen = time.Now()
		shard.mu.Unlock()

		if !pkl.limiter.Allow() {
			// Integrated Framework Response: Keeps status tracking visible to your logger
			c.Response.WriteHeader(http.StatusTooManyRequests)
			_, _ = c.Response.Write(zen.StringToBytes("Rate limit exceeded"))
			return
		}

		// Inject metrics into response headers using zero-allocation operations
		headers := c.Response.Header()
		headers.Set("X-RateLimit-Limit", burstStr)

		remainingTokens := int(math.Ceil(pkl.limiter.Tokens()))
		headers.Set("X-RateLimit-Remaining", strconv.Itoa(remainingTokens))

		c.Next()
	}
}
