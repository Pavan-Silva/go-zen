package zen

import (
	"errors"
	"fmt"
	"net/http"
	"reflect"
	"strconv"
	"strings"
)

// ErrInvalidBindTarget is returned by BindForm, BindPathParams, BindQueryParams,
// and bindFields when dest is not a pointer to a struct.
var ErrInvalidBindTarget = errors.New("http: bind dest must be a pointer to a struct")

// FormError is returned by BindForm when a form value cannot be parsed into
// the target field's type (e.g., "abc" into an int field).
type FormError struct {
	Field string
	Err   error
}

// Error implements the error interface.
func (e *FormError) Error() string {
	return "BindForm field \"" + e.Field + "\": " + e.Err.Error()
}

// Unwrap returns the underlying cause error.
func (e *FormError) Unwrap() error { return e.Err }

// Bind auto-detects the request content type and decodes the payload into dest.
// For GET, DELETE, and HEAD requests, it automatically populates query parameters first.
func (c *Ctx) Bind(dest any) error {
	if err := c.BindPathParams(dest); err != nil {
		return err
	}

	method := c.Request.Method
	if method == "GET" || method == "DELETE" || method == "HEAD" {
		if err := c.BindQueryParams(dest); err != nil {
			return err
		}
	}

	if _, exists := c.Request.Header["Content-Type"]; exists {
		return c.BindBody(dest)
	}
	return nil
}

// BindBody parses the request body into dest based on the Content-Type header,
// completely skipping path and query parameters.
func (c *Ctx) BindBody(dest any) error {
	ct := ""
	if vals := c.Request.Header["Content-Type"]; len(vals) > 0 {
		ct = vals[0]
	}
	if ct == "" {
		return nil
	}

	before, _, _ := strings.Cut(ct, ";")
	ct = strings.ToLower(strings.TrimSpace(before))

	switch ct {
	case "application/json":
		return c.BindJSON(dest)
	case "application/xml", "text/xml":
		return c.BindXML(dest)
	case "application/x-www-form-urlencoded", "multipart/form-data":
		return c.BindForm(dest)
	default:
		if strings.HasSuffix(ct, "+json") || strings.HasSuffix(ct, "/json") {
			return c.BindJSON(dest)
		}
		return fmt.Errorf("BindBody: unsupported content type %q", ct)
	}
}

// tagVal returns the first non-empty, non-"-" value from the given tag keys,
// stripped of comma options.
func tagVal(field reflect.StructField, keys ...string) string {
	for _, key := range keys {
		v := field.Tag.Get(key)
		if v == "" || v == "-" {
			continue
		}
		if idx := strings.IndexByte(v, ','); idx != -1 {
			v = v[:idx]
		}
		return v
	}
	return ""
}

// paramTag extracts the param tag, falling back to json then field name.
func paramTag(field reflect.StructField) string {
	if v := tagVal(field, "param", "json"); v != "" {
		return v
	}
	return field.Name
}

// queryTag extracts the query tag, falling back to json then field name.
func queryTag(field reflect.StructField) string {
	if v := tagVal(field, "query", "json"); v != "" {
		return v
	}
	return field.Name
}

// formTag extracts the form tag, falling back to json. Returns empty if neither exists.
func formTag(field reflect.StructField) string {
	return tagVal(field, "form", "json")
}

// headerTag extracts the header tag, falling back to json then canonical field name.
func headerTag(field reflect.StructField) string {
	if v := tagVal(field, "header", "json"); v != "" {
		return v
	}
	return http.CanonicalHeaderKey(field.Name)
}

// bindFields iterates over exported struct fields and applies a getter using the
// tag name from one of the given tag keys.
func (c *Ctx) bindFields(dest any, getter func(string) string, errPrefix string, tagKeys ...string) error {
	rv := reflect.ValueOf(dest)
	if rv.Kind() != reflect.Pointer || rv.Elem().Kind() != reflect.Struct {
		return ErrInvalidBindTarget
	}

	rv = rv.Elem()
	rt := rv.Type()

	for i := range rt.NumField() {
		field := rt.Field(i)
		if !field.IsExported() {
			continue
		}

		var name string
		switch tagKeys[0] {
		case "param":
			name = paramTag(field)
		case "query":
			name = queryTag(field)
		case "header":
			name = headerTag(field)
		default:
			name = tagVal(field, tagKeys...)
			if name == "" {
				continue
			}
		}

		fv := rv.Field(i)
		val := getter(name)
		if val == "" {
			continue
		}
		if err := setFieldValue(fv, field.Type.Kind(), []string{val}); err != nil {
			return fmt.Errorf("%s %q: %w", errPrefix, name, err)
		}
	}
	return nil
}

// BindPathParams extracts route wildcards using native Go 1.22+ request context maps.
func (c *Ctx) BindPathParams(dest any) error {
	return c.bindFields(dest, c.Param, "path param", "param")
}

// BindQueryParams parses query parameters into dest struct fields.
func (c *Ctx) BindQueryParams(dest any) error {
	if c.Request.URL.RawQuery == "" {
		return nil
	}
	return c.bindFields(dest, c.QueryParam, "query param", "query")
}

// BindHeader binds HTTP request headers to a struct using `header` or `json` tags.
//
// Example:
//
//	type Headers struct {
//	    UserID string `header:"X-User-Id"`
//	    APIKey string `header:"X-Api-Key"`
//	    Rate   int    `header:"X-Rate-Limit"`
//	}
func (c *Ctx) BindHeader(dest any) error {
	return c.bindFields(dest,
		func(name string) string { return c.Request.Header.Get(name) },
		"header", "header",
	)
}

// BindForm parses URL-encoded or multipart form data into a struct.
//
// Uses the "form" tag as primary source, falling back to "json" then field name.
// Automatically runs struct validation after binding unless validation was explicitly
// disabled via the setup configuration.
//
// Example:
//
//	type LoginForm struct {
//	    Email    string `form:"email" validate:"required,email"`
//	    Password string `form:"password" validate:"required,min=8"`
//	}
func (c *Ctx) BindForm(dest any) error {
	if c.Request.Form == nil {
		if err := c.Request.ParseForm(); err != nil {
			return fmt.Errorf("http: ParseForm error: %w", err)
		}
	}

	rv, rt, err := checkPointerToStruct(dest)
	if err != nil {
		return err
	}

	for i := range rt.NumField() {
		field := rt.Field(i)
		if !field.IsExported() {
			continue
		}

		key := formTag(field)
		if key == "" {
			continue
		}
		vals, ok := c.Request.Form[key]
		if !ok || len(vals) == 0 {
			continue
		}

		fv := rv.Field(i)
		if err := setFieldValue(fv, field.Type.Kind(), vals); err != nil {
			return &FormError{Field: key, Err: err}
		}
	}

	if autoValidateOn() {
		return Validate(dest)
	}
	return nil
}

// checkPointerToStruct validates dest is a pointer to a struct and returns the
// reflected value and type.
func checkPointerToStruct(dest any) (reflect.Value, reflect.Type, error) {
	rv := reflect.ValueOf(dest)
	if rv.Kind() != reflect.Pointer || rv.Elem().Kind() != reflect.Struct {
		return reflect.Value{}, nil, ErrInvalidBindTarget
	}
	rv = rv.Elem()
	return rv, rv.Type(), nil
}

// setFieldValue safely converts raw string slices into the concrete target field types.
func setFieldValue(fv reflect.Value, kind reflect.Kind, vals []string) error {
	val := vals[0]

	switch kind {
	case reflect.String:
		fv.SetString(val)
	case reflect.Slice:
		elemKind := fv.Type().Elem().Kind()
		switch elemKind {
		case reflect.String:
			fv.Set(reflect.ValueOf(vals))
		case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
			slice := reflect.MakeSlice(fv.Type(), len(vals), len(vals))
			for i, v := range vals {
				n, err := strconv.ParseInt(v, 10, 64)
				if err != nil {
					return err
				}
				slice.Index(i).SetInt(n)
			}
			fv.Set(slice)
		case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
			slice := reflect.MakeSlice(fv.Type(), len(vals), len(vals))
			for i, v := range vals {
				n, err := strconv.ParseUint(v, 10, 64)
				if err != nil {
					return err
				}
				slice.Index(i).SetUint(n)
			}
			fv.Set(slice)
		case reflect.Float32, reflect.Float64:
			slice := reflect.MakeSlice(fv.Type(), len(vals), len(vals))
			for i, v := range vals {
				n, err := strconv.ParseFloat(v, 64)
				if err != nil {
					return err
				}
				slice.Index(i).SetFloat(n)
			}
			fv.Set(slice)
		case reflect.Bool:
			slice := reflect.MakeSlice(fv.Type(), len(vals), len(vals))
			for i, v := range vals {
				b, err := strconv.ParseBool(v)
				if err != nil {
					return err
				}
				slice.Index(i).SetBool(b)
			}
			fv.Set(slice)
		default:
			return fmt.Errorf("unsupported slice element type %v", elemKind)
		}
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		n, err := strconv.ParseInt(val, 10, 64)
		if err != nil {
			return err
		}
		fv.SetInt(n)
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		n, err := strconv.ParseUint(val, 10, 64)
		if err != nil {
			return err
		}
		fv.SetUint(n)
	case reflect.Float32, reflect.Float64:
		n, err := strconv.ParseFloat(val, 64)
		if err != nil {
			return err
		}
		fv.SetFloat(n)
	case reflect.Bool:
		b, err := strconv.ParseBool(val)
		if err != nil {
			return err
		}
		fv.SetBool(b)
	default:
		return fmt.Errorf("unsupported bind type %v for field", kind)
	}
	return nil
}
