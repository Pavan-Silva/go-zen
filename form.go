package zen

import (
	"fmt"
	"reflect"
	"strconv"
	"strings"
	"sync"
)

// fieldInfo caches the reflection metadata needed to bind form fields.
// Computing this once per struct type avoids repeated reflection on every request.
type fieldInfo struct {
	index   int
	kind    reflect.Kind
	formKey string
	setFunc func(reflect.Value, string) error
}

// formCache stores pre-computed field metadata keyed by struct type.
// Using sync.Map allows lock-free reads after initial population.
var formCache sync.Map

// getFormFields returns the cached field metadata for the given struct type.
// On first call for a type, it inspects all exported fields, resolves their
// JSON tag names (stripping options like ",omitempty"), and creates a setFunc
// closure for each field. Subsequent calls return the cached slice.
func getFormFields(t reflect.Type) []fieldInfo {
	if cached, ok := formCache.Load(t); ok {
		return cached.([]fieldInfo)
	}

	numField := t.NumField()
	fields := make([]fieldInfo, 0, numField)

	for i := 0; i < numField; i++ {
		field := t.Field(i)

		// Skip unexported fields (PkgPath != "") and embedded fields.
		if !field.Anonymous && field.PkgPath == "" {
			tag := field.Tag.Get("json")
			if idx := strings.IndexByte(tag, ','); idx != -1 {
				tag = tag[:idx]
			}
			if tag == "" || tag == "-" {
				tag = field.Name
			}

			fields = append(fields, fieldInfo{
				index:   i,
				kind:    field.Type.Kind(),
				formKey: tag,
				setFunc: getSetFunc(field.Type.Kind()),
			})
		}
	}

	formCache.Store(t, fields)
	return fields
}

// getSetFunc returns a closure that sets a reflect.Value from a string.
// The closure is specific to one kind so the type switch is evaluated once
// at cache-build time rather than on every form field on every request.
func getSetFunc(kind reflect.Kind) func(reflect.Value, string) error {
	return func(fv reflect.Value, raw string) error {
		switch kind {
		case reflect.String:
			fv.SetString(raw)
		case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
			n, err := strconv.ParseInt(raw, 10, 64)
			if err != nil {
				return err
			}
			fv.SetInt(n)
		case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
			n, err := strconv.ParseUint(raw, 10, 64)
			if err != nil {
				return err
			}
			fv.SetUint(n)
		case reflect.Float32, reflect.Float64:
			f, err := strconv.ParseFloat(raw, 64)
			if err != nil {
				return err
			}
			fv.SetFloat(f)
		case reflect.Bool:
			// Check common truthy variants directly rather than calling
			// strings.ToLower, which allocates a new string on every call.
			switch raw {
			case "true", "TRUE", "True", "1", "yes", "YES", "Yes", "on", "ON", "On":
				fv.SetBool(true)
			default:
				fv.SetBool(false)
			}
		default:
			// Unsupported kinds are intentionally skipped; the field keeps its
			// zero value. No error is returned to stay consistent with the
			// "unknown keys are ignored" contract of mapFormValues.
		}
		return nil
	}
}

// BindForm parses the request's form data into dest, then runs struct
// validation if dest is a pointer to a struct.
//
// Both URL-encoded bodies (application/x-www-form-urlencoded) and multipart
// form data (multipart/form-data) are supported — Go's [http.Request.ParseForm]
// handles both transparently.
//
// Field mapping follows the same convention as [Context.BindJSON]: the "json"
// struct tag is used to match form keys to struct fields, falling back to the
// exported field name when no tag is present. This keeps the two binders
// consistent so the same struct can serve both JSON and form endpoints.
//
//	type SignupForm struct {
//	    Username string `json:"username" validate:"required,alphanum"`
//	    Age      int    `json:"age"      validate:"required,gte=18"`
//	}
//
// Possible error types returned:
//   - A wrapped [http.Request.ParseForm] error for malformed request bodies.
//   - [ErrInvalidBindTarget] if dest is not a pointer to a struct.
//   - [*FormError] if a form value cannot be converted to the field's type.
//   - A validation error from [github.com/go-playground/validator/v10] if a
//     validate tag constraint is violated.
func (c *Context) BindForm(dest any) error {
	if err := c.Request.ParseForm(); err != nil {
		return fmt.Errorf("zen: ParseForm error: %w", err)
	}

	if err := mapFormValues(c.Request.Form, dest); err != nil {
		return err
	}

	rv := reflect.ValueOf(dest)
	if rv.Kind() == reflect.Ptr {
		rv = rv.Elem()
	}
	if rv.Kind() == reflect.Struct {
		return validatorInstance().Struct(dest)
	}
	return nil
}

// mapFormValues maps form key-value pairs onto the exported fields of the
// struct pointed to by dest.
//
// Field names are resolved in this order:
//  1. The "json" struct tag (minus any options after a comma, e.g. omitempty)
//  2. The exported field name, if no usable tag is present
//
// Fields tagged with JSON:"-" are always skipped. Unexported fields are
// skipped automatically because [reflect.Value.CanSet] returns false for them.
// Form keys that do not match any field are silently ignored.
func mapFormValues(values map[string][]string, dest any) error {
	rv := reflect.ValueOf(dest)
	if rv.Kind() != reflect.Ptr || rv.Elem().Kind() != reflect.Struct {
		return ErrInvalidBindTarget
	}

	rv = rv.Elem()
	fields := getFormFields(rv.Type())

	for _, fi := range fields {
		vals, ok := values[fi.formKey]
		if !ok || len(vals) == 0 {
			continue
		}

		if err := fi.setFunc(rv.Field(fi.index), vals[0]); err != nil {
			return &FormError{Field: fi.formKey, Err: err}
		}
	}
	return nil
}
