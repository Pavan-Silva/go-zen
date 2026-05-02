package zen

import (
	"errors"
	"fmt"
	"reflect"
	"strconv"
	"strings"
)

// ErrInvalidBindTarget is returned by BindForm when dest is not a pointer to a struct.
var ErrInvalidBindTarget = errors.New("http: BindForm dest must be a pointer to a struct")

// FormError is returned by BindForm when a form value cannot be parsed into
// the target field's type (e.g., "abc" into an int field).
type FormError struct {
	Field string
	Err   error
}

// BindForm parses URL-encoded or multipart form data into a struct, then validates it.
// It uses "form" struct tags first, falling back to "json" tags.
//
// Supported field types: string, bool, all int/uint variants, float32, float64, []string.
//
// Example:
//
//	type LoginForm struct {
//	    Email    string `form:"email" validate:"required,email"`
//	    Password string `form:"password" validate:"required,min=8"`
//	}
//	var form LoginForm
//	if err := c.BindForm(&form); err != nil {
//	    // handle error
//	}
func (c *Context) BindForm(dest any) error {
	if err := c.Request.ParseForm(); err != nil {
		return fmt.Errorf("http: ParseForm error: %w", err)
	}

	rv := reflect.ValueOf(dest)
	if rv.Kind() != reflect.Pointer || rv.Elem().Kind() != reflect.Struct {
		return ErrInvalidBindTarget
	}

	rv = rv.Elem()
	rt := rv.Type()

	for i := 0; i < rt.NumField(); i++ {
		field := rt.Field(i)
		if field.PkgPath != "" {
			continue
		}

		tag := field.Tag.Get("form")
		if tag == "" || tag == "-" {
			tag = field.Tag.Get("json")
		}
		if tag == "" || tag == "-" {
			tag = field.Name
		}
		if idx := strings.IndexByte(tag, ','); idx != -1 {
			tag = tag[:idx]
		}

		vals, ok := c.Request.Form[tag]
		if !ok || len(vals) == 0 {
			continue
		}

		fv := rv.Field(i)
		if err := setFieldValue(fv, field.Type.Kind(), vals); err != nil {
			return &FormError{Field: tag, Err: err}
		}
	}

	return validateStruct(dest)
}

// setFieldValue converts string values to the target field type.
func setFieldValue(fv reflect.Value, kind reflect.Kind, vals []string) error {
	switch kind {
	case reflect.String:
		fv.SetString(vals[0])
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		n, err := strconv.ParseInt(vals[0], 10, 64)
		if err != nil {
			return err
		}
		fv.SetInt(n)
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		n, err := strconv.ParseUint(vals[0], 10, 64)
		if err != nil {
			return err
		}
		fv.SetUint(n)
	case reflect.Float32, reflect.Float64:
		f, err := strconv.ParseFloat(vals[0], 64)
		if err != nil {
			return err
		}
		fv.SetFloat(f)
	case reflect.Bool:
		switch vals[0] {
		case "true", "TRUE", "True", "1", "yes", "YES", "Yes", "on", "ON", "On":
			fv.SetBool(true)
		default:
			fv.SetBool(false)
		}
	case reflect.Slice:
		if fv.Type().Elem().Kind() == reflect.String {
			fv.Set(reflect.ValueOf(vals))
		}
	}
	return nil
}

// Error implements the error interface.
func (e *FormError) Error() string {
	return "BindForm field \"" + e.Field + "\": " + e.Err.Error()
}

// Unwrap returns the underlying cause error.
func (e *FormError) Unwrap() error { return e.Err }
