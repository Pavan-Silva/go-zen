package zen

import (
	"reflect"
	"strings"

	"github.com/go-playground/validator/v10"
)

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

// defaultValidate is the built-in validator using go-playground/validator/v10.
type defaultValidate struct {
	inst *validator.Validate
}

// ValidatorFunc is a helper type to turn a plain function into a Validator.
type ValidatorFunc func(i any) error

// Validate implements the Validator interface for ValidatorFunc.
func (f ValidatorFunc) Validate(i any) error {
	return f(i)
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

// --- Ctx validation ---

// validateIfEnabled runs automated struct validation when the engine has
// auto-validation enabled and a validator is configured.
func (e *Engine) validateIfEnabled(dest any) error {
	if e.autoValidate {
		return e.Validate(dest)
	}
	return nil
}

// Validate runs struct validation on dest using the engine's configured validator.
// Returns nil if no validator is set (validation is opt-in).
func (c *Ctx) Validate(dest any) error {
	return c.engine.Validate(dest)
}
