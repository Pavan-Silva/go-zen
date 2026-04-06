package env

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

// Load reads a .env-style file and sets environment variables.
// Lines beginning with # are ignored. If override is false, existing
// environment variables are preserved.
// It looks for .env file by default.
func Load(paths ...string) error {
	if len(paths) == 0 {
		paths = []string{".env"}
	}

	for _, path := range paths {
		if err := loadFile(path, false); err != nil {
			return err
		}
	}
	return nil
}

// LoadOverride reads a .env-style file and sets environment variables,
// overriding existing values.
func LoadOverride(paths ...string) error {
	if len(paths) == 0 {
		paths = []string{".env"}
	}

	for _, path := range paths {
		if err := loadFile(path, true); err != nil {
			return err
		}
	}
	return nil
}

func loadFile(path string, override bool) (err error) {
	f, err := os.Open(path)
	if err != nil {
		return err
	}

	defer func() {
		closeErr := f.Close()
		if err == nil && closeErr != nil {
			err = closeErr
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
			return fmt.Errorf("invalid env line %q", line)
		}

		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])
		value = trimQuotes(value)

		if key == "" {
			return fmt.Errorf("invalid env key in line %q", line)
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

// Get returns the environment value for key or defaultValue if not set.
func Get(key, defaultValue string) string {
	if value, ok := os.LookupEnv(key); ok {
		return value
	}
	return defaultValue
}

// Must return the environment value for key or panics if it is not set.
func Must(key string) string {
	value, ok := os.LookupEnv(key)
	if !ok {
		panic(fmt.Sprintf("required environment variable %q is not set", key))
	}
	return value
}

// GetInt parses an environment variable as an int, falling back to defaultValue.
func GetInt(key string, defaultValue int) (int, error) {
	value := Get(key, "")
	if value == "" {
		return defaultValue, nil
	}
	return strconv.Atoi(value)
}

// GetBool parses an environment variable as a bool, falling back to defaultValue.
func GetBool(key string, defaultValue bool) (bool, error) {
	value := Get(key, "")
	if value == "" {
		return defaultValue, nil
	}
	return strconv.ParseBool(value)
}

// GetDuration parses an environment variable as a time.Duration, falling back to defaultValue.
func GetDuration(key string, defaultValue time.Duration) (time.Duration, error) {
	value := Get(key, "")
	if value == "" {
		return defaultValue, nil
	}
	return time.ParseDuration(value)
}
