package zen

import (
	"fmt"
	"net/http"
	"reflect"
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
		if err := setFieldValue(fv, f.kind, []string{headerValue}); err != nil {
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
