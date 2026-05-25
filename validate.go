package zen

import (
	"reflect"
	"strings"
	"sync"

	"github.com/go-playground/validator/v10"
)

// validateTagCache tracks which struct types have "validate" struct tags,
// so we can skip the full validator pipeline for types without any rules.
var validateTagCache sync.Map

// Validator is the interface for request validation.
// Implement this to plug in any validation library.
//
// Example:
//
//	type CustomValidator struct {
//	    validator *validator.Validate
//	}
//
//	func (cv *CustomValidator) Validate(i any) error {
//	    return cv.validator.Struct(i)
//	}
type Validator interface {
	Validate(i any) error
}

// defaultValidator is the global validator instance.
// Set a custom validator via SetValidator().
var defaultValidator Validator = &defaultValidate{inst: newValidator()}

// defaultValidate is the built-in validator using go-playground/validator/v10.
type defaultValidate struct {
	inst *validator.Validate
}

// SetValidator sets a custom validator for request validation.
// Pass nil to disable validation entirely.
func SetValidator(v Validator) {
	defaultValidator = v
}

// DisableAutoValidation disables automatic request validation for BindJSON,
// BindXML, and BindForm. This is useful for performance-sensitive handlers where
// the request payload is already trusted or validation is performed elsewhere.
func DisableAutoValidation() {
	SetValidator(nil)
}

// Validate implements the Validator interface for defaultValidate.
func (v *defaultValidate) Validate(i any) error {
	rv := reflect.ValueOf(i)
	if rv.Kind() == reflect.Pointer {
		rv = rv.Elem()
	}
	if rv.Kind() != reflect.Struct {
		return nil
	}
	return v.inst.Struct(i)
}

// newValidator creates and configures the default go-playground/validator instance.
func newValidator() *validator.Validate {
	inst := validator.New()
	inst.SetTagName("validate")

	// Configures error messages to use the structural JSON name tags instead of Go field names
	inst.RegisterTagNameFunc(func(fld reflect.StructField) string {
		tag := fld.Tag.Get("json")
		if tag == "" || tag == "-" {
			return ""
		}
		if i := strings.IndexByte(tag, ','); i != -1 {
			tag = tag[:i]
		}
		return tag
	})

	return inst
}

// typeHasValidateTags checks whether a struct type has any "validate" struct tags.
// Results are cached to avoid repeated reflection scans on the hot path.
func typeHasValidateTags(t reflect.Type) bool {
	if v, ok := validateTagCache.Load(t); ok {
		return v.(bool)
	}
	for i := 0; i < t.NumField(); i++ {
		if t.Field(i).Tag.Get("validate") != "" {
			validateTagCache.Store(t, true)
			return true
		}
	}
	validateTagCache.Store(t, false)
	return false
}

// Validate runs struct validation on dest using the configured Validator.
// BindJSON, BindXML, and BindForm call this automatically after decoding.
func Validate(dest any) error {
	if defaultValidator == nil {
		return nil
	}

	// Only skip tag-based validation for the default built-in validator.
	// Custom validators may have logic beyond struct tags.
	if _, ok := defaultValidator.(*defaultValidate); ok {
		rv := reflect.ValueOf(dest)
		if rv.Kind() == reflect.Pointer {
			rv = rv.Elem()
		}
		if rv.Kind() == reflect.Struct && !typeHasValidateTags(rv.Type()) {
			return nil
		}
	}

	return defaultValidator.Validate(dest)
}
