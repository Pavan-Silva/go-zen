package zen

import (
	"fmt"
	"reflect"
	"strconv"
	"strings"
)

// BindForm parses the request form into dest and runs validation.
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

// mapFormValues uses reflection to map form keys to struct fields.
func mapFormValues(values map[string][]string, dest any) error {
	rv := reflect.ValueOf(dest)
	if rv.Kind() != reflect.Ptr || rv.Elem().Kind() != reflect.Struct {
		return fmt.Errorf("zen: BindForm dest must be a pointer to a struct")
	}

	rv = rv.Elem()
	rt := rv.Type()

	for i := 0; i < rt.NumField(); i++ {
		field := rt.Field(i)
		fieldVal := rv.Field(i)

		// Optimization: Check if field is exportable
		if !fieldVal.CanSet() {
			continue
		}

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
			return fmt.Errorf("zen: BindForm field %q: %w", tag, err)
		}
	}
	return nil
}

// setField handles the type conversion from string to the struct field type.
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
		// Avoid allocations from strings.ToLower by checking common variants directly.
		switch raw {
		case "true", "TRUE", "True", "1", "yes", "YES", "Yes", "on", "ON", "On":
			fv.SetBool(true)
		default:
			fv.SetBool(false)
		}

	default:
		// Unsupported types are skipped to keep it "Zen" (minimal)
	}
	return nil
}
