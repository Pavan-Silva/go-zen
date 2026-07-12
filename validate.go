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

// --- Engine validation methods ---

// SetValidator sets a custom validator for request validation on this engine.
// Pass nil to disable validation entirely.
func (e *Engine) SetValidator(v Validator) {
	e.validator = v
}

// EnableAutoValidation enables automatic Validate() calls after
// BindJSON, BindXML, and BindForm for this engine.
func (e *Engine) EnableAutoValidation() {
	e.autoValidate = true
}

// Validate runs struct validation on dest using the engine's configured validator.
// Returns nil if no validator is set (validation is opt-in).
func (e *Engine) Validate(dest any) error {
	if e.validator == nil {
		return nil
	}
	return e.validator.Validate(dest)
}

// DefaultValidator returns the underlying go-playground/validator/v10 instance
// when using the default built-in validator, or nil if a custom validator is set.
// Use this to register custom validation tags:
//
//	e.DefaultValidator().RegisterValidation("is-even", func(fl validator.FieldLevel) bool {
//	    return fl.Field().Int()%2 == 0
//	})
func (e *Engine) DefaultValidator() *validator.Validate {
	if dv, ok := e.validator.(*defaultValidate); ok {
		return dv.inst
	}
	return nil
}

// --- Ctx validation ---

// Validate runs struct validation on dest using the engine's configured validator.
// Returns nil if no validator is set (validation is opt-in).
func (c *Ctx) Validate(dest any) error {
	if c.engine.validator == nil {
		return nil
	}
	return c.engine.validator.Validate(dest)
}
