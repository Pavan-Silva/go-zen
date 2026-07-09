package zen

import (
	"errors"
	"fmt"
	"reflect"
	"strings"
	"sync"
)

// ErrInvalidBindTarget is returned by BindForm when dest is not a pointer to a struct.
var ErrInvalidBindTarget = errors.New("http: BindForm dest must be a pointer to a struct")

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

// formField represents a pre-compiled, optimized mapping for a form field.
type formField struct {
	index   int
	formTag string
	kind    reflect.Kind
}

// Global thread-safe cache to store compiled form struct layouts.
var formCache sync.Map

// BindForm parses URL-encoded or multipart form data into a struct.
//
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
	// ParseMultipartForm or ParseForm caches the parsed results natively.
	// If it was already parsed (e.g., by multipart handlers), this is a no-op.
	if c.Request.Form == nil {
		if err := c.Request.ParseForm(); err != nil {
			return fmt.Errorf("http: ParseForm error: %w", err)
		}
	}

	rv := reflect.ValueOf(dest)
	if rv.Kind() != reflect.Pointer || rv.Elem().Kind() != reflect.Struct {
		return ErrInvalidBindTarget
	}

	rv = rv.Elem()
	rt := rv.Type()

	// Fast Path: Load pre-compiled fields or compile them on the very first miss
	var fields []formField
	if cached, ok := formCache.Load(rt); ok {
		fields = cached.([]formField)
	} else {
		fields = compileFormStruct(rt)
		formCache.Store(rt, fields)
	}

	// Hot Path Execution Loop
	for i := 0; i < len(fields); i++ {
		field := fields[i]
		vals, ok := c.Request.Form[field.formTag]
		if !ok || len(vals) == 0 {
			continue
		}

		fv := rv.Field(field.index)
		if err := setFieldValue(fv, field.kind, vals); err != nil {
			return &FormError{Field: field.formTag, Err: err}
		}
	}

	if autoValidateOn() {
		return Validate(dest)
	}
	return nil
}

// compileFormStruct parses struct tags once at boot time / first request.
func compileFormStruct(t reflect.Type) []formField {
	numFields := t.NumField()
	fields := make([]formField, 0, numFields)

	for i := range numFields {
		f := t.Field(i)
		if f.PkgPath != "" && !f.Anonymous {
			continue
		}

		key := f.Tag.Get("form")
		if key == "" {
			key = f.Tag.Get("json")
		}
		if key == "" || key == "-" {
			continue
		}

		if idx := strings.IndexByte(key, ','); idx != -1 {
			key = key[:idx]
		}

		fields = append(fields, formField{
			index:   i,
			formTag: key,
			kind:    f.Type.Kind(),
		})
	}
	return fields
}
