package zen

import (
	"encoding/json"
	"io"
	"log"
	"reflect"
)

func (c *Context) BindJSON(dest any) error {
	defer func(Body io.ReadCloser) {
		err := Body.Close()
		if err != nil {
			log.Printf("zen: BindJSON body close error: %v", err)
		}
	}(c.Request.Body)

	// Direct stream decoding is more performant than buffering
	if err := json.NewDecoder(c.Request.Body).Decode(dest); err != nil {
		return err
	}

	// Validation logic
	val := reflect.ValueOf(dest)
	if val.Kind() == reflect.Ptr {
		val = val.Elem()
	}
	if val.Kind() == reflect.Struct {
		return validatorInstance().Struct(dest)
	}
	return nil
}

func (c *Context) QueryParam(key string) string {
	if c.queryCache == nil {
		c.queryCache = c.Request.URL.Query()
	}
	return c.queryCache.Get(key)
}

func (c *Context) Param(key string) string {
	return c.Request.PathValue(key)
}

func (c *Context) Body() ([]byte, error) {
	defer func(Body io.ReadCloser) {
		err := Body.Close()
		if err != nil {
			log.Printf("zen: Body close error: %v", err)
		}
	}(c.Request.Body)
	return io.ReadAll(c.Request.Body)
}
