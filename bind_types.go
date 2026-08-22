package zen

import (
	"encoding"
	"errors"
	"reflect"
	"strconv"
	"time"
)

// setWithProperType sets structField to the parsed value of val based on
// valueKind. It handles all Go primitive kinds (int*, uint*, float*, bool,
// string), pointer fields (auto-initialized before recursive descent), and
// returns an error for unknown kinds.
func setWithProperType(valueKind reflect.Kind, val string, structField reflect.Value) error {
	switch valueKind {
	case reflect.Pointer:
		if structField.IsNil() {
			structField.Set(reflect.New(structField.Type().Elem()))
		}
		return setWithProperType(structField.Elem().Kind(), val, structField.Elem())
	case reflect.Int:
		return setIntField(val, 0, structField)
	case reflect.Int8:
		return setIntField(val, 8, structField)
	case reflect.Int16:
		return setIntField(val, 16, structField)
	case reflect.Int32:
		return setIntField(val, 32, structField)
	case reflect.Int64:
		return setIntField(val, 64, structField)
	case reflect.Uint:
		return setUintField(val, 0, structField)
	case reflect.Uint8:
		return setUintField(val, 8, structField)
	case reflect.Uint16:
		return setUintField(val, 16, structField)
	case reflect.Uint32:
		return setUintField(val, 32, structField)
	case reflect.Uint64:
		return setUintField(val, 64, structField)
	case reflect.Bool:
		return setBoolField(val, structField)
	case reflect.Float32:
		return setFloatField(val, 32, structField)
	case reflect.Float64:
		return setFloatField(val, 64, structField)
	case reflect.String:
		structField.SetString(val)
	default:
		return errors.New("unknown type")
	}
	return nil
}

// unmarshalInputsToField checks whether field implements
// BindMultipleUnmarshaler and, if so, calls UnmarshalParams with the entire
// values slice. It returns (true, err) when the interface was implemented
// and (false, nil) when it was not. Pointer fields are auto-initialized
// before the interface check.
func unmarshalInputsToField(valueKind reflect.Kind, values []string, field reflect.Value) (bool, error) {
	if valueKind == reflect.Pointer {
		if field.IsNil() {
			field.Set(reflect.New(field.Type().Elem()))
		}
		field = field.Elem()
	}

	fieldIValue := field.Addr().Interface()
	unmarshaler, ok := fieldIValue.(BindMultipleUnmarshaler)
	if !ok {
		return false, nil
	}
	return true, unmarshaler.UnmarshalParams(values)
}

// unmarshalInputToField checks whether field implements BindUnmarshaler or
// encoding.TextUnmarshaler and, if so, delegates to the appropriate method.
// When formatTag is set and the field is time.Time the value is parsed with
// time.Parse using that format. It returns (true, result) when a custom
// unmarshaler was used and (false, nil) when the field should fall through
// to the primitive setter. Pointer fields are auto-initialized before the
// interface check.
func unmarshalInputToField(valueKind reflect.Kind, val string, field reflect.Value, formatTag string) (bool, error) {
	if valueKind == reflect.Pointer {
		if field.IsNil() {
			field.Set(reflect.New(field.Type().Elem()))
		}
		field = field.Elem()
	}

	fieldIValue := field.Addr().Interface()
	if formatTag != "" {
		if _, isTime := fieldIValue.(*time.Time); isTime {
			t, err := time.Parse(formatTag, val)
			if err != nil {
				return true, err
			}
			field.Set(reflect.ValueOf(t))
			return true, nil
		}
	}

	switch unmarshaler := fieldIValue.(type) {
	case BindUnmarshaler:
		return true, unmarshaler.UnmarshalParam(val)
	case encoding.TextUnmarshaler:
		return true, unmarshaler.UnmarshalText([]byte(val))
	}

	return false, nil
}

// setIntField parses val as a signed integer of the given bitSize and sets
// field to the result. An empty string is treated as "0".
func setIntField(value string, bitSize int, field reflect.Value) error {
	if value == "" {
		value = "0"
	}
	intVal, err := strconv.ParseInt(value, 10, bitSize)
	if err == nil {
		field.SetInt(intVal)
	}
	return err
}

// setUintField parses val as an unsigned integer of the given bitSize and
// sets field to the result. An empty string is treated as "0".
func setUintField(value string, bitSize int, field reflect.Value) error {
	if value == "" {
		value = "0"
	}
	uintVal, err := strconv.ParseUint(value, 10, bitSize)
	if err == nil {
		field.SetUint(uintVal)
	}
	return err
}

// setBoolField parses val as a boolean and sets field to the result. An
// empty string is treated as "false".
func setBoolField(value string, field reflect.Value) error {
	if value == "" {
		value = "false"
	}
	boolVal, err := strconv.ParseBool(value)
	if err == nil {
		field.SetBool(boolVal)
	}
	return err
}

// setFloatField parses val as a floating-point number of the given bitSize
// and sets field to the result. An empty string is treated as "0.0".
func setFloatField(value string, bitSize int, field reflect.Value) error {
	if value == "" {
		value = "0.0"
	}
	floatVal, err := strconv.ParseFloat(value, bitSize)
	if err == nil {
		field.SetFloat(floatVal)
	}
	return err
}
