package zen

import (
	"fmt"
	"reflect"
	"strconv"
	"strings"
)

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

	val := reflect.ValueOf(dest)
	if val.Kind() == reflect.Ptr {
		val = val.Elem()
	}
	if val.Kind() == reflect.Struct {
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
	rt := rv.Type()

	for i := 0; i < rt.NumField(); i++ {
		field := rt.Field(i)
		fieldVal := rv.Field(i)

		// Skip unexported fields — reflect will panic on SetXxx if we don't.
		if !fieldVal.CanSet() {
			continue
		}

		// Resolve the form key from the JSON tag, stripping tag options such
		// as ",omitempty". Fall back to the field name when no tag is set.
		tag := field.Tag.Get("json")
		if idx := strings.IndexByte(tag, ','); idx != -1 {
			tag = tag[:idx]
		}
		if tag == "" || tag == "-" {
			tag = field.Name
		}

		vals, ok := values[tag]
		if !ok || len(vals) == 0 {
			continue
		}

		if err := setField(fieldVal, vals[0]); err != nil {
			return &FormError{Field: tag, Err: err}
		}
	}
	return nil
}

// setField converts the raw form string value into the kind of the target
// struct field and writes it using reflection.
//
// Supported kinds: string, int*, uint*, float*, bool.
// Unsupported kinds (slices, maps, nested structs, etc.) are intentionally
// skipped and the field retains its zero value. This keeps the implementation
// minimal; use [Context.BindJSON] for complex nested payloads.
//
// Bool parsing accepts the following truthy literals without allocating:
// "true", "TRUE", "True", "1", "yes", "YES", "Yes", "on", "ON", "On".
// Any other value is treated as false.
func setField(fv reflect.Value, raw string) error {
	switch fv.Kind() {
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
