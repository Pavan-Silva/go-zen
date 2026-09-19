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
	if v, ok := lookupEnv(key, strconv.Atoi); ok {
		return v
	}
	return defaultValue
}

// GetBool parses the environment variable as bool or returns default.
func GetBool(key string, defaultValue bool) bool {
	if v, ok := lookupEnv(key, strconv.ParseBool); ok {
		return v
	}
	return defaultValue
}

// GetDuration parses the environment variable as time.Duration or returns default.
func GetDuration(key string, defaultValue time.Duration) time.Duration {
	if v, ok := lookupEnv(key, time.ParseDuration); ok {
		return v
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
	return mustEnv(key, strconv.Atoi, "int")
}

// MustGetBool parses the environment variable as bool or panics.
func MustGetBool(key string) bool {
	return mustEnv(key, strconv.ParseBool, "bool")
}

// MustGetDuration parses the environment variable as time.Duration or panics.
func MustGetDuration(key string) time.Duration {
	return mustEnv(key, time.ParseDuration, "duration")
}

// lookupEnv returns the parsed value of an environment variable and whether
// it is set and parseable. An absent, empty, or invalid value yields the zero
// value and false, so callers fall back to their default.
func lookupEnv[T any](key string, parse func(string) (T, error)) (T, bool) {
	var parsed T
	v := os.Getenv(key)
	if v == "" {
		return parsed, false
	}
	parsed, err := parse(v)
	if err != nil {
		return parsed, false
	}
	return parsed, true
}

// mustEnv parses a required environment variable of the given kind, panicking
// when it is unset or cannot be parsed.
func mustEnv[T any](key string, parse func(string) (T, error), kind string) T {
	v := MustGetString(key)
	parsed, err := parse(v)
	if err != nil {
		panic(fmt.Sprintf("environment variable %s is not a valid %s: %s", key, kind, v))
	}
	return parsed
}
