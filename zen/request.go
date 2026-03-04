package zen

import (
	"encoding/json"
	"io"
	"log"
	"reflect"
)

// BindJSON decodes the request body as JSON into dest and runs struct validation.
// Direct stream decoding avoids an intermediate buffer allocation.
func (c *Context) BindJSON(dest any) error {
	defer func(Body io.ReadCloser) {
		if err := Body.Close(); err != nil {
			log.Printf("zen: BindJSON body close error: %v", err)
		}
	}(c.Request.Body)

	if err := json.NewDecoder(c.Request.Body).Decode(dest); err != nil {
		return err
	}

	val := reflect.ValueOf(dest)
	if val.Kind() == reflect.Ptr {
		val = val.Elem()
	}
	if val.Kind() == reflect.Struct {
		return validatorInstance().Struct(dest)
	}
	return nil
}

// QueryParam returns the first value for the named query parameter.
// The parsed query string is cached on first access.
func (c *Context) QueryParam(key string) string {
	if c.queryCache == nil {
		c.queryCache = c.Request.URL.Query()
	}
	return c.queryCache.Get(key)
}

// Param returns the path parameter value for the given key (Go 1.22+ PathValue).
func (c *Context) Param(key string) string {
	return c.Request.PathValue(key)
}

// Body reads and returns the full raw request body.
func (c *Context) Body() ([]byte, error) {
	defer func(Body io.ReadCloser) {
		if err := Body.Close(); err != nil {
			log.Printf("zen: Body close error: %v", err)
		}
	}(c.Request.Body)
	return io.ReadAll(c.Request.Body)
}
