package middleware

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/Pavan-Silva/go-zen"
	"github.com/Pavan-Silva/go-zen/internal/bytesconv"
)

// BodyLimitConfig holds configuration for request body size limiting.
type BodyLimitConfig struct {
	// Limit can be an int, int64, or a string like "2M", "250K", "1G".
	Limit   any
	Skipper zen.SkipFunc // Optional function to skip body limiting for certain requests.
}

// DefaultBodyLimitConfig returns a config with a 2MB default limit.
func DefaultBodyLimitConfig() BodyLimitConfig {
	return BodyLimitConfig{
		Limit: "2M",
	}
}

// BodyLimit returns middleware that limits request body size.
func BodyLimit(limit any) zen.HandlerFunc {
	return BodyLimitWithConfig(BodyLimitConfig{Limit: limit})
}

// BodyLimitWithConfig returns middleware that limits request body size using a config.
func BodyLimitWithConfig(config BodyLimitConfig) zen.HandlerFunc {
	// Parse limit during initialization.
	computedLimit, err := parseLimitToBytes(config.Limit)
	if err != nil {
		panic(fmt.Sprintf("BodyLimit compilation failure: %v", err))
	}

	return func(c *zen.Ctx) {
		if config.Skipper != nil && config.Skipper(c.Request) {
			c.Next()
			return
		}

		// Check ContentLength before wrapping the body reader.
		if c.Request.ContentLength > computedLimit {
			c.Response.WriteHeader(http.StatusRequestEntityTooLarge)
			_, _ = c.Response.Write(bytesconv.StringToBytes(http.StatusText(http.StatusRequestEntityTooLarge)))
			return
		}

		// Wrap body with MaxBytesReader.
		c.Request.Body = http.MaxBytesReader(c.Response, c.Request.Body, computedLimit)

		c.Next()
	}
}

// parseLimitToBytes converts a limit value to bytes.
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
