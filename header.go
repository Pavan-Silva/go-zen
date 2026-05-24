package zen

import (
	"fmt"
	"net/http"
	"reflect"
	"strconv"
	"strings"
	"sync"
)

// headerField represents a pre-compiled, optimized mapping for a struct field.
type headerField struct {
	index     int
	headerKey string
	kind      reflect.Kind
}

// Global thread-safe cache to store compiled struct layouts.
// This ensures reflect tag scanning happens exactly once per unique struct type.
var headerCache sync.Map

// BindHeader binds HTTP request headers to a struct using `header` or `json` tags.
// Header names are canonicalized via http.CanonicalHeaderKey at compilation time.
//
// Example:
//
//	type Headers struct {
//	    UserID string `header:"X-User-Id"`
//	    APIKey string `header:"X-Api-Key"`
//	    Rate   int    `header:"X-Rate-Limit"`
//	}
func (c *Ctx) BindHeader(dest any) error {
	v := reflect.ValueOf(dest)
	if v.Kind() != reflect.Pointer || v.Elem().Kind() != reflect.Struct {
		return fmt.Errorf("header: dest must be a pointer to struct")
	}

	v = v.Elem()
	t := v.Type()

	// Fast Path: Check if this struct type layout has already been compiled
	var fields []headerField
	if cached, ok := headerCache.Load(t); ok {
		fields = cached.([]headerField)
	} else {
		// Cold Path: Compile struct metadata once and save it
		fields = compileHeaderStruct(t)
		headerCache.Store(t, fields)
	}

	// Hot Path Execution: Zero tag lookups, direct array iterations
	for i := 0; i < len(fields); i++ {
		f := fields[i]
		headerValue := c.Request.Header.Get(f.headerKey)
		if headerValue == "" {
			continue
		}

		fv := v.Field(f.index)
		if err := setHeaderField(fv, f.kind, headerValue); err != nil {
			return fmt.Errorf("header %q: %w", f.headerKey, err)
		}
	}

	return nil
}

// compileHeaderStruct parses struct metadata once and extracts fast-path bindings.
func compileHeaderStruct(t reflect.Type) []headerField {
	numFields := t.NumField()
	fields := make([]headerField, 0, numFields)

	for i := range numFields {
		f := t.Field(i)
		// Skip unexported fields
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

		// Strip tag options like omitEmpty or comma flags
		if idx := strings.IndexByte(key, ','); idx != -1 {
			key = key[:idx]
		}

		fields = append(fields, headerField{
			index:     i,
			headerKey: key,
			kind:      f.Type.Kind(),
		})
	}
	return fields
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
	default:
	}
	return nil
}
