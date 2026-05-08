package zen

import (
	"errors"
	"fmt"
	"reflect"
)

// ErrInvalidBindTarget is returned by BindForm when dest is not a pointer to a struct.
var ErrInvalidBindTarget = errors.New("http: BindForm dest must be a pointer to a struct")

// FormError is returned by BindForm when a form value cannot be parsed into
// the target field's type (e.g., "abc" into an int field).
type FormError struct {
	Field string
	Err   error
}

// BindForm parses URL-encoded or multipart form data into a struct.
//
// Automatically runs struct validation after binding if a Validator is configured.
//
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

		tag := parseStructTag(field, "form")

		vals, ok := c.Request.Form[tag]
		if !ok || len(vals) == 0 {
			continue
		}

		fv := rv.Field(i)
		if err := setFieldValue(fv, field.Type.Kind(), vals); err != nil {
			return &FormError{Field: tag, Err: err}
		}
	}

	return Validate(dest)
}

// Error implements the error interface.
func (e *FormError) Error() string {
	return "BindForm field \"" + e.Field + "\": " + e.Err.Error()
}

// Unwrap returns the underlying cause error.
func (e *FormError) Unwrap() error { return e.Err }
