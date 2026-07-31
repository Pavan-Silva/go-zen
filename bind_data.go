package zen

import (
	"encoding"
	"errors"
	"fmt"
	"mime/multipart"
	"net/http"
	"reflect"
	"strings"
)

// maxBindDepth bounds how deep bindDataValue recurses into nested struct
// fields. Without it, a self-referencing struct (e.g. type Node struct {
// Next *Node }) would recurse indefinitely and crash the process with a stack
// overflow.
const maxBindDepth = 64

func bindData(destination any, data map[string][]string, tag string, dataFiles map[string][]*multipart.FileHeader) error {
	if destination == nil {
		return ErrInvalidBindTarget
	}
	v := reflect.ValueOf(destination)
	if v.Kind() != reflect.Pointer || v.IsNil() {
		return ErrInvalidBindTarget
	}
	typ := v.Type().Elem()
	if typ.Kind() != reflect.Struct && typ.Kind() != reflect.Map {
		return ErrInvalidBindTarget
	}
	if len(data) == 0 && len(dataFiles) == 0 {
		return nil
	}
	return bindDataValue(typ, v.Elem(), data, tag, dataFiles, 0)
}

func bindDataValue(typ reflect.Type, val reflect.Value, data map[string][]string, tag string, dataFiles map[string][]*multipart.FileHeader, depth int) error {
	if depth > maxBindDepth {
		return fmt.Errorf("binding exceeded maximum recursion depth of %d (circular reference?)", maxBindDepth)
	}
	hasFiles := len(dataFiles) > 0

	if typ.Kind() == reflect.Map {
		if typ.Key().Kind() != reflect.String {
			return fmt.Errorf("unsupported map key type %s (must be string)", typ.Key().String())
		}
		elemKind := typ.Elem().Kind()
		switch elemKind {
		case reflect.String:
			if val.IsNil() {
				val.Set(reflect.MakeMap(typ))
			}
			for k, v := range data {
				val.SetMapIndex(reflect.ValueOf(k), reflect.ValueOf(strings.Join(v, ",")))
			}
		case reflect.Interface:
			if val.IsNil() {
				val.Set(reflect.MakeMap(typ))
			}
			for k, v := range data {
				val.SetMapIndex(reflect.ValueOf(k), reflect.ValueOf(strings.Join(v, ",")))
			}
		case reflect.Slice:
			if typ.Elem().Elem().Kind() != reflect.String {
				return fmt.Errorf("unsupported map slice element type %s", typ.Elem().Elem().String())
			}
			if val.IsNil() {
				val.Set(reflect.MakeMap(typ))
			}
			for k, v := range data {
				val.SetMapIndex(reflect.ValueOf(k), reflect.ValueOf(v))
			}
		default:
			return fmt.Errorf("unsupported map value type %s", typ.Elem().String())
		}
		return nil
	}

	if typ.Kind() != reflect.Struct {
		return errors.New("binding element must be a struct")
	}

	for i := range typ.NumField() {
		sf := typ.Field(i)
		structField := val.Field(i)

		explicitTag := sf.Tag.Get(tag)
		jsonName := jsonTagName(sf.Tag)
		formatTag := sf.Tag.Get("format")

		var inputFieldName string
		var hasBindingName bool
		if explicitTag != "" {
			inputFieldName = explicitTag
			hasBindingName = true
		} else if jsonName != "" {
			inputFieldName = jsonName
			hasBindingName = true
		} else {
			inputFieldName = sf.Name
		}

		structFieldKind := structField.Kind()
		realKind := structFieldKind
		if realKind == reflect.Pointer {
			realKind = structField.Type().Elem().Kind()
		}

		if sf.Anonymous && realKind == reflect.Struct && !hasBindingName {
			if structField.Kind() == reflect.Pointer {
				if structField.IsNil() {
					if structField.CanSet() {
						structField.Set(reflect.New(structField.Type().Elem()))
					} else {
						continue
					}
				}
				structField = structField.Elem()
			}
			if structField.CanSet() {
				if _, ok := structField.Addr().Interface().(BindUnmarshaler); ok {
					continue
				}
				if _, ok := structField.Addr().Interface().(BindMultipleUnmarshaler); ok {
					continue
				}
				if err := bindDataValue(structField.Type(), structField, data, tag, dataFiles, depth+1); err != nil {
					return err
				}
			}
			continue
		}

		if sf.Anonymous && realKind == reflect.Struct && hasBindingName {
			return errors.New("query/param/form/header tags are not allowed with anonymous struct field")
		}

		if realKind == reflect.Struct && !sf.Anonymous {
			hasUnmarshaler := false
			if _, ok := structField.Interface().(BindUnmarshaler); ok {
				hasUnmarshaler = true
			} else if structField.CanAddr() {
				if _, ok := structField.Addr().Interface().(BindUnmarshaler); ok {
					hasUnmarshaler = true
				}
			}
			if _, ok := structField.Interface().(encoding.TextUnmarshaler); ok {
				hasUnmarshaler = true
			} else if structField.CanAddr() {
				if _, ok := structField.Addr().Interface().(encoding.TextUnmarshaler); ok {
					hasUnmarshaler = true
				}
			}

			if !hasUnmarshaler {
				tempField := structField
				if tempField.Kind() == reflect.Pointer {
					if tempField.IsNil() {
						if tempField.CanSet() {
							tempField.Set(reflect.New(tempField.Type().Elem()))
						} else {
							continue
						}
					}
					tempField = tempField.Elem()
				}
				if tempField.CanSet() {
					if err := bindDataValue(tempField.Type(), tempField, data, tag, dataFiles, depth+1); err != nil {
						return err
					}
				}
				continue
			}
		}

		if !structField.CanSet() {
			continue
		}

		if hasFiles {
			if ok, err := isFieldMultipartFile(structField.Type()); err != nil {
				return err
			} else if ok {
				if ok := setMultipartFileHeaderTypes(structField, inputFieldName, dataFiles); ok {
					continue
				}
			}
		}

		var inputValue []string
		var exists bool
		if tag == "header" {
			inputValue, exists = data[http.CanonicalHeaderKey(inputFieldName)]
		} else {
			inputValue, exists = data[inputFieldName]
		}

		if !exists {
			for k, v := range data {
				if strings.EqualFold(k, inputFieldName) {
					inputValue = v
					exists = true
					break
				}
			}
		}

		if !exists {
			continue
		}

		if len(inputValue) == 0 {
			continue
		}

		if ok, err := unmarshalInputsToField(sf.Type.Kind(), inputValue, structField); ok {
			if err != nil {
				return fmt.Errorf("%s: %w", inputFieldName, err)
			}
			continue
		}

		if ok, err := unmarshalInputToField(sf.Type.Kind(), inputValue[0], structField, formatTag); ok {
			if err != nil {
				return fmt.Errorf("%s: %w", inputFieldName, err)
			}
			continue
		}

		if structFieldKind == reflect.Pointer {
			if structField.IsNil() {
				if structField.CanSet() {
					structField.Set(reflect.New(structField.Type().Elem()))
				} else {
					continue
				}
			}
			structFieldKind = structField.Elem().Kind()
			structField = structField.Elem()
		}

		if structFieldKind == reflect.Slice {
			numElems := len(inputValue)
			slice := reflect.MakeSlice(structField.Type(), numElems, numElems)
			elemType := structField.Type().Elem()
			elemKind := elemType.Kind()
			for j := range numElems {
				elemVal := slice.Index(j)
				ok, err := unmarshalInputToField(elemKind, inputValue[j], elemVal, formatTag)
				if ok {
					if err != nil {
						return fmt.Errorf("%s: %w", inputFieldName, err)
					}
					continue
				}
				if err := setWithProperType(elemKind, inputValue[j], elemVal); err != nil {
					return fmt.Errorf("%s: %w", inputFieldName, err)
				}
			}
			structField.Set(slice)
			continue
		}

		if structFieldKind == reflect.Map {
			mt := structField.Type()
			if mt.Key().Kind() != reflect.String {
				return fmt.Errorf("unsupported map key type %s (must be string)", mt.Key().String())
			}
			if structField.IsNil() {
				structField.Set(reflect.MakeMap(mt))
			}
			elemKind := mt.Elem().Kind()
			switch elemKind {
			case reflect.String:
				structField.SetMapIndex(reflect.ValueOf(inputFieldName), reflect.ValueOf(strings.Join(inputValue, ",")))
			case reflect.Interface:
				structField.SetMapIndex(reflect.ValueOf(inputFieldName), reflect.ValueOf(strings.Join(inputValue, ",")))
			case reflect.Slice:
				if mt.Elem().Elem().Kind() != reflect.String {
					return fmt.Errorf("unsupported map slice element type %s", mt.Elem().Elem().String())
				}
				structField.SetMapIndex(reflect.ValueOf(inputFieldName), reflect.ValueOf(inputValue))
			default:
				return fmt.Errorf("unsupported map value type %s", mt.Elem().String())
			}
			continue
		}

		if err := setWithProperType(structFieldKind, inputValue[0], structField); err != nil {
			return fmt.Errorf("%s: %w", inputFieldName, err)
		}
	}
	return nil
}

// jsonTagName extracts the JSON field name from a struct tag. It handles
// comma-separated options (e.g. "name,omitempty" returns "name") and
// returns an empty string when the tag is empty or set to "-".
func jsonTagName(tag reflect.StructTag) string {
	name := tag.Get("json")
	if name == "" || name == "-" {
		return ""
	}
	if idx := strings.IndexByte(name, ','); idx != -1 {
		name = name[:idx]
	}
	return name
}
