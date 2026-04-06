package config

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

// LoadEnvFile reads a .env-style file and sets environment variables.
// Lines beginning with # are ignored. If override is false, existing
// environment variables are preserved.
func LoadEnvFile(path string, override bool) (err error) {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer func() {
		if errClose := f.Close(); errClose != nil {
			if err != nil {
				err = fmt.Errorf("%w; close error: %v", err, errClose)
			} else {
				err = errClose
			}
		}
	}()

	s := bufio.NewScanner(f)
	for s.Scan() {
		line := strings.TrimSpace(s.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			return fmt.Errorf("util: invalid env line %q", line)
		}

		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])
		value = trimQuotes(value)

		if key == "" {
			return fmt.Errorf("util: invalid env key in line %q", line)
		}

		if !override {
			if _, ok := os.LookupEnv(key); ok {
				continue
			}
		}

		if err := os.Setenv(key, value); err != nil {
			return err
		}
	}

	return s.Err()
}

func trimQuotes(value string) string {
	if len(value) >= 2 {
		if strings.HasPrefix(value, `"`) && strings.HasSuffix(value, `"`) {
			return strings.Trim(value, `"`)
		}
		if strings.HasPrefix(value, "'") && strings.HasSuffix(value, "'") {
			return strings.Trim(value, "'")
		}
	}
	return value
}

// GetEnv returns the environment value for key or defaultValue if not set.
func GetEnv(key, defaultValue string) string {
	if value, ok := os.LookupEnv(key); ok {
		return value
	}
	return defaultValue
}

// MustGetEnv returns the environment value for key or panics if it is not set.
func MustGetEnv(key string) string {
	value, ok := os.LookupEnv(key)
	if !ok {
		panic(fmt.Sprintf("util: required environment variable %q is not set", key))
	}
	return value
}

// GetEnvInt parses an environment variable as an int, falling back to defaultValue.
func GetEnvInt(key string, defaultValue int) (int, error) {
	value := GetEnv(key, "")
	if value == "" {
		return defaultValue, nil
	}
	return strconv.Atoi(value)
}

// GetEnvBool parses an environment variable as a bool, falling back to defaultValue.
func GetEnvBool(key string, defaultValue bool) (bool, error) {
	value := GetEnv(key, "")
	if value == "" {
		return defaultValue, nil
	}
	return strconv.ParseBool(value)
}

// GetEnvDuration parses an environment variable as a time.Duration, falling back to defaultValue.
func GetEnvDuration(key string, defaultValue time.Duration) (time.Duration, error) {
	value := GetEnv(key, "")
	if value == "" {
		return defaultValue, nil
	}
	return time.ParseDuration(value)
}
