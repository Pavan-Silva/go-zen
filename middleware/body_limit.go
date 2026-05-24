package middleware

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/Pavan-Silva/go-zen"
)

// BodyLimitConfig holds configuration parameters for the request volume firewall.
type BodyLimitConfig struct {
	// Limit can be a raw byte integer value (int/int64) or a string metric suffix like "2M", "250K", "1G".
	Limit   any
	Skipper zen.SkipFunc
}

// DefaultBodyLimitConfig returns a standard configuration template (Default: 2 Megabytes).
func DefaultBodyLimitConfig() BodyLimitConfig {
	return BodyLimitConfig{
		Limit: "2M",
	}
}

// BodyLimit protects downstream routing handlers from accommodating bloated request payloads.
func BodyLimit(limit any) zen.HandlerFunc {
	return BodyLimitWithConfig(BodyLimitConfig{Limit: limit})
}

// BodyLimitWithConfig initializes the size boundary check using a structural configuration object.
func BodyLimitWithConfig(config BodyLimitConfig) zen.HandlerFunc {
	// Parse the interface input into a flat int64 byte scalar value once during startup
	computedLimit, err := parseLimitToBytes(config.Limit)
	if err != nil {
		panic(fmt.Sprintf("BodyLimit compilation failure: %v", err))
	}

	return func(c *zen.Ctx) {
		if config.Skipper != nil && config.Skipper(c.Request) {
			c.Next()
			return
		}

		// Hardened Fast-Path Verification: Verify content metrics if provided by the client.
		// A negative ContentLength indicates a chunked execution stream (-1), which we bypass here
		// and allow MaxBytesReader to catch safely at the driver layer below.
		if c.Request.ContentLength > computedLimit {
			c.Response.WriteHeader(http.StatusRequestEntityTooLarge)
			_, _ = c.Response.Write(zen.StringToBytes(http.StatusText(http.StatusRequestEntityTooLarge)))
			return
		}

		// Wrap the native connection scanner context to break the stream if it crosses the limit
		c.Request.Body = http.MaxBytesReader(c.Response, c.Request.Body, computedLimit)

		c.Next()
	}
}

// parseLimitToBytes resolves raw ints, int64s, or string metric rules safely down to primitive byte numbers.
func parseLimitToBytes(limit any) (int64, error) {
	switch v := limit.(type) {
	case int64:
		return v, nil
	case int:
		return int64(v), nil
	case string:
		return parseStringMetric(v)
	default:
		return 0, fmt.Errorf("unsupported limit type definition (must be string, int, or int64)")
	}
}

func parseStringMetric(limit string) (int64, error) {
	limit = strings.TrimSpace(limit)
	if limit == "" {
		return 0, fmt.Errorf("empty constraint declaration")
	}

	var multiplier int64 = 1
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

	value, err := strconv.ParseInt(limit, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("failed to extract base metric: %w", err)
	}

	return value * multiplier, nil
}
