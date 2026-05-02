package zen

import (
	"fmt"
	"net/http"
	"reflect"
	"strconv"
	"strings"
	"sync"
)

// headerCache caches struct field information for header binding.
var headerCache sync.Map

// headerFieldInfo holds metadata for a struct field when binding from headers.
type headerFieldInfo struct {
	index   int
	key     string
	setFunc func(reflect.Value, string) error
}

// getHeaderFields extracts header field mappings from a struct type.
func getHeaderFields(t reflect.Type) []headerFieldInfo {
	if cached, ok := headerCache.Load(t); ok {
		return cached.([]headerFieldInfo)
	}

	var fields []headerFieldInfo
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

		// Remove options like ",omitempty"
		if idx := strings.Index(key, ","); idx != -1 {
			key = key[:idx]
		}

		info := headerFieldInfo{
			index:   i,
			key:     key,
			setFunc: makeHeaderSetter(f.Type.Kind()),
		}
		fields = append(fields, info)
	}

	headerCache.Store(t, fields)
	return fields
}

// makeHeaderSetter creates a function to set a reflect.Value from a header string.
func makeHeaderSetter(k reflect.Kind) func(reflect.Value, string) error {
	switch k {
	case reflect.String:
		return func(v reflect.Value, s string) error {
			v.SetString(s)
			return nil
		}
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return func(v reflect.Value, s string) error {
			n, err := strconv.ParseInt(s, 10, 64)
			if err != nil {
				return err
			}
			v.SetInt(n)
			return nil
		}
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return func(v reflect.Value, s string) error {
			n, err := strconv.ParseUint(s, 10, 64)
			if err != nil {
				return err
			}
			v.SetUint(n)
			return nil
		}
	case reflect.Float32, reflect.Float64:
		return func(v reflect.Value, s string) error {
			n, err := strconv.ParseFloat(s, 64)
			if err != nil {
				return err
			}
			v.SetFloat(n)
			return nil
		}
	case reflect.Bool:
		return func(v reflect.Value, s string) error {
			b, err := strconv.ParseBool(s)
			if err != nil {
				return err
			}
			v.SetBool(b)
			return nil
		}
	default:
		return func(v reflect.Value, s string) error {
			return nil
		}
	}
}

// BindHeader binds HTTP request headers to a struct using `header` or `json` tags.
// Header names are canonicalized (e.g., "user-id" becomes "User-Id").
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
	fields := getHeaderFields(t)

	for _, f := range fields {
		headerValue := c.Request.Header.Get(f.key)
		if headerValue == "" {
			continue
		}
		f.setFunc(v.Field(f.index), headerValue)
	}

	return validateStruct(dest)
}

// MustBindHeader binds headers and panics on error.
func (c *Context) MustBindHeader(dest any) {
	if err := c.BindHeader(dest); err != nil {
		panic(err)
	}
}
