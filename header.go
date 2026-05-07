package zen

import (
	"fmt"
	"net/http"
	"reflect"
	"strconv"
	"strings"
)

// BindHeader binds HTTP request headers to a struct using `header` or `json` tags.
// Header names are canonicalized via http.CanonicalHeaderKey.
//
// Supported field types: string, bool, all int/uint variants, float32, float64.
// Struct tags follow the same pattern as BindForm and BindQueryParams:
// the "header" tag is checked first, then "json", then the field name.
//
// Example:
//
//	type Headers struct {
//	    UserID string `header:"X-User-Id"`
//	    APIKey string `header:"X-Api-Key"`
//	    Rate   int    `header:"X-Rate-Limit"`
//	}
//
//	var h Headers
//	if err := c.BindHeader(&h); err != nil {
//	    c.Error(http.StatusBadRequest, "invalid headers")
//	    return
//	}
func (c *Context) BindHeader(dest any) error {
	v := reflect.ValueOf(dest)
	if v.Kind() != reflect.Pointer || v.Elem().Kind() != reflect.Struct {
		return fmt.Errorf("header: dest must be a pointer to struct")
	}

	v = v.Elem()
	t := v.Type()

	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)
		if f.PkgPath != "" && !f.Anonymous {
			continue
		}

		key := f.Tag.Get("header")
		if key == "" {
			key = f.Tag.Get("json")
		}
		if key == "" {
			key = http.CanonicalHeaderKey(f.Name)
		}
		if key == "-" {
			continue
		}

		if idx := strings.IndexByte(key, ','); idx != -1 {
			key = key[:idx]
		}

		headerValue := c.Request.Header.Get(key)
		if headerValue == "" {
			continue
		}

		fv := v.Field(i)
		if err := setHeaderField(fv, f.Type.Kind(), headerValue); err != nil {
			return fmt.Errorf("header %q: %w", key, err)
		}
	}

	return nil
}

// setHeaderField converts a header string value to the target field type.
func setHeaderField(fv reflect.Value, kind reflect.Kind, s string) error {
	switch kind {
	case reflect.String:
		fv.SetString(s)
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		n, err := strconv.ParseInt(s, 10, 64)
		if err != nil {
			return err
		}
		fv.SetInt(n)
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		n, err := strconv.ParseUint(s, 10, 64)
		if err != nil {
			return err
		}
		fv.SetUint(n)
	case reflect.Float32, reflect.Float64:
		n, err := strconv.ParseFloat(s, 64)
		if err != nil {
			return err
		}
		fv.SetFloat(n)
	case reflect.Bool:
		b, err := strconv.ParseBool(s)
		if err != nil {
			return err
		}
		fv.SetBool(b)
	}
	return nil
}
