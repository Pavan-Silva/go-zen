package openapi

import (
	"reflect"
	"strings"
	"time"
)

// schemaBuilder converts Go types to OpenAPI Schema Objects via reflection.
type schemaBuilder struct {
	schemas map[string]any          // named schema registry
	seen    map[reflect.Type]string // visited types -> schema name
}

// newSchemaBuilder creates a schemaBuilder.
func newSchemaBuilder() *schemaBuilder {
	return &schemaBuilder{
		schemas: make(map[string]any),
		seen:    make(map[reflect.Type]string),
	}
}

// schemaOf returns an OpenAPI Schema Object for the given value's type.
func (sb *schemaBuilder) schemaOf(v any) map[string]any {
	t := reflect.TypeOf(v)
	if t == nil {
		return map[string]any{"type": "object"}
	}

	for t.Kind() == reflect.Pointer {
		t = t.Elem()
	}

	return sb.typeToSchema(t)
}

// typeToSchema converts a reflect.Type to an OpenAPI Schema Object.
func (sb *schemaBuilder) typeToSchema(t reflect.Type) map[string]any {
	switch t.Kind() {
	case reflect.Pointer:
		return sb.typeToSchema(t.Elem())
	case reflect.Struct:
		if t == reflect.TypeFor[time.Time]() {
			return map[string]any{"type": "string", "format": "date-time"}
		}

		if name := t.Name(); name != "" {
			if _, ok := sb.seen[t]; ok {
				return map[string]any{"$ref": "#/components/schemas/" + name}
			}
			sb.registerStruct(t)
			return map[string]any{"$ref": "#/components/schemas/" + name}
		}

		return sb.inlineStruct(t)
	case reflect.Slice, reflect.Array:
		elem := t.Elem()
		return map[string]any{
			"type":  "array",
			"items": sb.typeToSchema(elem),
		}
	case reflect.Map:
		return map[string]any{
			"type":                 "object",
			"additionalProperties": sb.typeToSchema(t.Elem()),
		}
	default:
		return map[string]any{"type": oapiType(t)}
	}
}

// registerStruct registers a named struct type in the schema registry.
func (sb *schemaBuilder) registerStruct(t reflect.Type) {
	name := t.Name()
	if name == "" {
		return
	}

	if _, ok := sb.seen[t]; ok {
		return
	}

	sb.seen[t] = name

	schema := sb.inlineStruct(t)
	sb.schemas[name] = schema
}

// inlineStruct converts an anonymous or named struct to an inline schema.
func (sb *schemaBuilder) inlineStruct(t reflect.Type) map[string]any {
	schema := map[string]any{
		"type":       "object",
		"properties": make(map[string]any),
	}
	var required []string

	for i := range t.NumField() {
		f := t.Field(i)
		if !f.IsExported() {
			continue
		}

		if f.Anonymous && f.Type.Kind() == reflect.Struct {
			sb.mergeInlineFields(schema, f.Type)
			continue
		}

		name, prop := sb.fieldToProperty(f)
		if name == "" {
			continue
		}

		schema["properties"].(map[string]any)[name] = prop
		if f.Type.Kind() != reflect.Struct && f.Type.Kind() != reflect.Pointer {
			if isRequired(f) {
				required = append(required, name)
			}
		}
	}

	if len(required) > 0 {
		schema["required"] = required
	}

	return schema
}

// mergeInlineFields merges fields from an embedded struct into the parent schema.
func (sb *schemaBuilder) mergeInlineFields(schema map[string]any, t reflect.Type) {
	var required []string

	for i := range t.NumField() {
		f := t.Field(i)
		if !f.IsExported() {
			continue
		}

		name, prop := sb.fieldToProperty(f)
		if name == "" {
			continue
		}

		schema["properties"].(map[string]any)[name] = prop
		if f.Type.Kind() != reflect.Struct && f.Type.Kind() != reflect.Pointer {
			if isRequired(f) {
				required = append(required, name)
			}
		}
	}

	if len(required) > 0 {
		if existing, ok := schema["required"]; ok {
			schema["required"] = append(existing.([]string), required...)
		} else {
			schema["required"] = required
		}
	}
}

// fieldToProperty converts a struct field to a property name and schema.
func (sb *schemaBuilder) fieldToProperty(f reflect.StructField) (string, map[string]any) {
	jsonTag := f.Tag.Get("json")
	name := f.Name
	if jsonTag != "" {
		parts := strings.Split(jsonTag, ",")
		if parts[0] == "-" {
			return "", nil
		}

		if parts[0] != "" {
			name = parts[0]
		}
	}

	return name, sb.typeToSchema(f.Type)
}

// isRequired checks whether a struct field has a validate:"required" tag.
func isRequired(f reflect.StructField) bool {
	tag := f.Tag.Get("validate")
	if tag == "" {
		return false
	}

	for part := range strings.SplitSeq(tag, ",") {
		if strings.TrimSpace(part) == "required" {
			return true
		}
	}

	return false
}

// oapiType maps a Go kind to its OpenAPI type string.
func oapiType(t reflect.Type) string {
	switch t.Kind() {
	case reflect.String:
		return "string"
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return "integer"
	case reflect.Float32, reflect.Float64:
		return "number"
	case reflect.Bool:
		return "boolean"
	default:
		return "string"
	}
}

// build returns the collected named schemas.
func (sb *schemaBuilder) build() map[string]any {
	return sb.schemas
}
