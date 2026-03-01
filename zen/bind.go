package zen

import (
	"fmt"
	"log"
	"reflect"
	"strings"
)

// BindForm parses the request form into dest, then runs validation using
// binding struct tags. The returned error is either a decode error or a
// validator.ValidationErrors — callers can use errors.As to distinguish them.
// It uses a single reflection pass over the struct fields — no intermediate
// map, no JSON marshal/unmarshal round-trip.
func (c *Context) BindForm(dest interface{}) error {
	if c.Request.Form == nil {
		if err := c.Request.ParseForm(); err != nil {
			log.Printf("zen: ParseForm error: %v", err)
		}
	}
	if err := mapFormValues(c.Request.Form, dest); err != nil {
		return err
	}

	// Skip validation for non-structs
	val := reflect.ValueOf(dest)
	if val.Kind() == reflect.Ptr {
		val = val.Elem()
	}

	if val.Kind() == reflect.Struct {
		return validatorInstance().Struct(dest)
	}

	return nil
}

// mapFormValues walks the struct fields of dest once and sets each field
// whose JSON tag matches a key in values. Only string, int/int64, float64,
// and bool fields are handled — extend the type switch below if you need more.
func mapFormValues(values map[string][]string, dest interface{}) error {
	rv := reflect.ValueOf(dest)
	if rv.Kind() != reflect.Ptr || rv.Elem().Kind() != reflect.Struct {
		return fmt.Errorf("zen: BindForm dest must be a pointer to a struct")
	}
	rv = rv.Elem()
	rt := rv.Type()

	for i := 0; i < rt.NumField(); i++ {
		field := rt.Field(i)
		fieldVal := rv.Field(i)

		tag := field.Tag.Get("json")
		if i := strings.IndexByte(tag, ','); i != -1 {
			tag = tag[:i]
		}
		if tag == "" || tag == "-" {
			tag = field.Name
		}

		vals, ok := values[tag]
		if !ok || len(vals) == 0 {
			continue
		}

		if err := setField(fieldVal, vals[0]); err != nil {
			return fmt.Errorf("zen: BindForm field %q: %w", tag, err)
		}
	}
	return nil
}

// setField sets a single reflected struct field from a raw string value.
func setField(fv reflect.Value, raw string) error {
	switch fv.Kind() {
	case reflect.String:
		fv.SetString(raw)

	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		var n int64
		if _, err := fmt.Sscan(raw, &n); err != nil {
			return err
		}
		fv.SetInt(n)

	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		var n uint64
		if _, err := fmt.Sscan(raw, &n); err != nil {
			return err
		}
		fv.SetUint(n)

	case reflect.Float32, reflect.Float64:
		var f float64
		if _, err := fmt.Sscan(raw, &f); err != nil {
			return err
		}
		fv.SetFloat(f)

	case reflect.Bool:
		switch strings.ToLower(raw) {
		case "true", "1", "yes", "on":
			fv.SetBool(true)
		case "false", "0", "no", "off":
			fv.SetBool(false)
		default:
			return fmt.Errorf("invalid bool value %q", raw)
		}

	default:
		// silently skip types we don't handle (time.Time, nested structs, etc.)
		// caller can handle those fields manually after BindForm returns
	}
	return nil
}
