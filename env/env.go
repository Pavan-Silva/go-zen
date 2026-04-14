package env

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

// Load reads one or more .env-style files and sets environment variables.
// Lines beginning with # are treated as comments and ignored.
// Existing environment variables are preserved (not overridden).
// If no paths are provided, it defaults to ".env".
func Load(filenames ...string) error {
	return load(false, filenames...)
}

// Overload reads one or more .env-style files and sets environment variables,
// overriding any existing values.
// If no paths are provided, it defaults to ".env".
func Overload(filenames ...string) error {
	return load(true, filenames...)
}

// Read reads one or more .env-style files and returns the parsed key-value
// pairs as a map, without modifying the environment.
// If no paths are provided, it defaults to ".env".
func Read(filenames ...string) (map[string]string, error) {
	if len(filenames) == 0 {
		filenames = []string{".env"}
	}

	result := make(map[string]string)
	for _, filename := range filenames {
		pairs, err := parseFile(filename)
		if err != nil {
			return nil, err
		}
		for k, v := range pairs {
			result[k] = v
		}
	}
	return result, nil
}

// Parse parses an env-style string and returns the key-value pairs as a map,
// without modifying the environment.
func Parse(s string) (map[string]string, error) {
	return parseReader(strings.NewReader(s))
}

func load(override bool, filenames ...string) error {
	if len(filenames) == 0 {
		filenames = []string{".env"}
	}

	for _, filename := range filenames {
		pairs, err := parseFile(filename)
		if err != nil {
			return err
		}
		for k, v := range pairs {
			if !override {
				if _, exists := os.LookupEnv(k); exists {
					continue
				}
			}
			if err := os.Setenv(k, v); err != nil {
				return err
			}
		}
	}
	return nil
}

func parseFile(filename string) (map[string]string, error) {
	f, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	return parseReader(f)
}

func parseReader(r io.Reader) (map[string]string, error) {
	pairs := make(map[string]string)
	s := bufio.NewScanner(r)

	for s.Scan() {
		line := strings.TrimSpace(s.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		// Strip inline comments (e.g. KEY=value # comment)
		if idx := strings.Index(line, " #"); idx != -1 {
			line = strings.TrimSpace(line[:idx])
		}

		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			return nil, fmt.Errorf("env: invalid line %q", line)
		}

		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])

		if key == "" {
			return nil, fmt.Errorf("env: empty key in line %q", line)
		}

		pairs[key] = unquote(value)
	}

	if err := s.Err(); err != nil {
		return nil, err
	}
	return pairs, nil
}

func unquote(value string) string {
	if len(value) >= 2 {
		switch {
		case value[0] == '"' && value[len(value)-1] == '"':
			return value[1 : len(value)-1]
		case value[0] == '\'' && value[len(value)-1] == '\'':
			return value[1 : len(value)-1]
		}
	}
	return value
}

// Get returns the value of the environment variable named by key,
// or defaultValue if the variable is not set.
func Get(key, defaultValue string) string {
	if value, ok := os.LookupEnv(key); ok {
		return value
	}
	return defaultValue
}

// Must returns the value of the environment variable named by key.
// It panics if the variable is not set.
func Must(key string) string {
	value, ok := os.LookupEnv(key)
	if !ok {
		panic(fmt.Sprintf("env: required variable %q is not set", key))
	}
	return value
}

// GetInt parses the environment variable named by key as an int.
// Returns defaultValue if the variable is not set.
func GetInt(key string, defaultValue int) (int, error) {
	value, ok := os.LookupEnv(key)
	if !ok {
		return defaultValue, nil
	}
	return strconv.Atoi(value)
}

// GetBool parses the environment variable named by key as a bool.
// Returns defaultValue if the variable is not set.
// Accepts 1, t, T, TRUE, true, True, 0, f, F, FALSE, false, False.
func GetBool(key string, defaultValue bool) (bool, error) {
	value, ok := os.LookupEnv(key)
	if !ok {
		return defaultValue, nil
	}
	return strconv.ParseBool(value)
}

// GetDuration parses the environment variable named by key as a time.Duration.
// Returns defaultValue if the variable is not set.
// Accepts any duration string valid for time.ParseDuration (e.g. "300ms", "1h30m").
func GetDuration(key string, defaultValue time.Duration) (time.Duration, error) {
	value, ok := os.LookupEnv(key)
	if !ok {
		return defaultValue, nil
	}
	return time.ParseDuration(value)
}