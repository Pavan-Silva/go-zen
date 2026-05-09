package middleware

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/Pavan-Silva/go-zen"
)

// BodyLimitMiddlewareConfig holds configuration for BodyLimit middleware.
type BodyLimitMiddlewareConfig struct {
	// Limit is the maximum allowed request body size.
	// Can be a number (bytes) or with suffix: K, M, G (e.g., "2M", "1K").
	Limit string
	// Skipper defines a function to skip middleware.
	Skipper zen.SkipFunc
}

// BodyLimitConfig returns a config with default values.
func BodyLimitConfig() BodyLimitMiddlewareConfig {
	return BodyLimitMiddlewareConfig{
		Limit: "2M",
	}
}

// BodyLimit returns a middleware that limits the request body size.
// It wraps the request body with http.MaxBytesReader to enforce the limit.
// If the body exceeds the limit, it returns a 413 Request Entity Too Large response.
//
// This function uses the default configuration with the specified limit.
// The limit can be a string (e.g., "2M", "1K") or an int64 (bytes).
//
// Example:
//
//	r.Use(middleware.BodyLimit("2M"))
//	r.Use(middleware.BodyLimit(2_097_152))
func BodyLimit(limit any) zen.MiddlewareFunc {
	config := BodyLimitConfig()
	switch v := limit.(type) {
	case string:
		config.Limit = v
	case int64:
		config.Limit = strconv.FormatInt(v, 10)
	case int:
		config.Limit = strconv.Itoa(v)
	default:
		panic("BodyLimit: limit must be string or int64")
	}
	return BodyLimitWithConfig(config)
}

// BodyLimitWithConfig returns a middleware that limits the request body size.
// It wraps the request body with http.MaxBytesReader to enforce the limit.
// If the body exceeds the limit, it returns a 413 Request Entity Too Large response.
//
// The middleware can be skipped for certain routes using the Skipper function.
//
// Example:
//
//	r.Use(middleware.BodyLimitWithConfig(middleware.DefaultBodyLimitConfig()))
//
// Example with custom limit:
//
//	config := middleware.DefaultBodyLimitConfig()
//	config.Limit = "1M"
//	r.Use(middleware.BodyLimitWithConfig(config))
//
// Example with skipper:
//
//	config := middleware.DefaultBodyLimitConfig()
//	config.Skipper = func(r *http.Request) bool {
//		return r.URL.Path == "/upload/large"
//	}
//	r.Use(middleware.BodyLimitWithConfig(config))
func BodyLimitWithConfig(config BodyLimitMiddlewareConfig) zen.MiddlewareFunc {
	limit, err := parseLimit(config.Limit)
	if err != nil {
		panic(fmt.Sprintf("invalid body limit: %v", err))
	}

	return func(c *zen.Context, next zen.NextFunc) {
		if config.Skipper != nil && config.Skipper(c.Request) {
			next(c)
			return
		}

		if c.Request.ContentLength > limit {
			http.Error(c.Response, http.StatusText(http.StatusRequestEntityTooLarge), http.StatusRequestEntityTooLarge)
			return
		}

		c.Request.Body = http.MaxBytesReader(c.Response, c.Request.Body, limit)

		next(c)
	}
}

// parseLimit parses a size string like "2M", "1K", "512" into bytes.
func parseLimit(limit string) (int64, error) {
	limit = strings.TrimSpace(limit)
	if limit == "" {
		return 0, fmt.Errorf("empty limit")
	}

	// Check for suffix
	var multiplier int64 = 1
	if len(limit) > 1 {
		suffix := limit[len(limit)-1]
		switch suffix {
		case 'K', 'k':
			multiplier = 1024
			limit = limit[:len(limit)-1]
		case 'M', 'm':
			multiplier = 1024 * 1024
			limit = limit[:len(limit)-1]
		case 'G', 'g':
			multiplier = 1024 * 1024 * 1024
			limit = limit[:len(limit)-1]
		}
	}

	// Parse the number
	value, err := strconv.ParseInt(limit, 10, 64)
	if err != nil {
		return 0, err
	}

	return value * multiplier, nil
}
