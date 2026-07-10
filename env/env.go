// Package env provides helpers for reading configuration from environment variables.
package env

import (
	"fmt"
	"os"
	"strconv"
	"time"
)

// GetString returns the environment variable value or default if not set.
func GetString(key, defaultValue string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return defaultValue
}

// GetInt parses the environment variable as int or returns default.
func GetInt(key string, defaultValue int) int {
	if v := os.Getenv(key); v != "" {
		if i, err := strconv.Atoi(v); err == nil {
			return i
		}
	}
	return defaultValue
}

// GetBool parses the environment variable as bool or returns default.
func GetBool(key string, defaultValue bool) bool {
	if v := os.Getenv(key); v != "" {
		if b, err := strconv.ParseBool(v); err == nil {
			return b
		}
	}
	return defaultValue
}

// GetDuration parses the environment variable as time.Duration or returns default.
func GetDuration(key string, defaultValue time.Duration) time.Duration {
	if v := os.Getenv(key); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			return d
		}
	}
	return defaultValue
}

// MustGetString returns the environment variable or panics if not set.
func MustGetString(key string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	panic(fmt.Sprintf("required environment variable %s not set", key))
}

// MustGetInt parses the environment variable as int or panics.
func MustGetInt(key string) int {
	v := MustGetString(key)
	if i, err := strconv.Atoi(v); err == nil {
		return i
	}
	panic(fmt.Sprintf("environment variable %s is not a valid int: %s", key, v))
}

// MustGetBool parses the environment variable as bool or panics.
func MustGetBool(key string) bool {
	v := MustGetString(key)
	if b, err := strconv.ParseBool(v); err == nil {
		return b
	}
	panic(fmt.Sprintf("environment variable %s is not a valid bool: %s", key, v))
}

// MustGetDuration parses the environment variable as time.Duration or panics.
func MustGetDuration(key string) time.Duration {
	v := MustGetString(key)
	if d, err := time.ParseDuration(v); err == nil {
		return d
	}
	panic(fmt.Sprintf("environment variable %s is not a valid duration: %s", key, v))
}
