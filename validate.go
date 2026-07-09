package zen

import (
	"reflect"
	"strings"
	"sync"

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

// defaultValidator is the global validator instance.
// Set a custom validator via SetValidator().
var (
	defaultValidatorMu sync.RWMutex
	defaultValidator   Validator = &defaultValidate{inst: newValidator()}
)

// autoValidate controls whether BindJSON/BindXML/BindForm automatically
// call Validate after decoding. Disabled by default.
var (
	autoValidateMu      sync.RWMutex
	autoValidateEnabled bool
)

// defaultValidate is the built-in validator using go-playground/validator/v10.
type defaultValidate struct {
	inst *validator.Validate
}

// getValidator returns the current global validator.
func getValidator() Validator {
	defaultValidatorMu.RLock()
	v := defaultValidator
	defaultValidatorMu.RUnlock()
	return v
}

// SetValidator sets a custom validator for request validation.
// Pass nil to disable validation entirely.
func SetValidator(v Validator) {
	defaultValidatorMu.Lock()
	defaultValidator = v
	defaultValidatorMu.Unlock()
}

// EnableAutoValidation enables automatic Validate() calls after
// BindJSON, BindXML, and BindForm.
func EnableAutoValidation() {
	autoValidateMu.Lock()
	autoValidateEnabled = true
	autoValidateMu.Unlock()
}

// autoValidateOn returns whether auto-validation is enabled.
func autoValidateOn() bool {
	autoValidateMu.RLock()
	v := autoValidateEnabled
	autoValidateMu.RUnlock()
	return v
}

// DefaultValidator returns the underlying go-playground/validator/v10 instance
// when using the default built-in validator, or nil if a custom validator is set.
// Use this to register custom validation tags:
//
//	zen.DefaultValidator().RegisterValidation("is-even", func(fl validator.FieldLevel) bool {
//	    return fl.Field().Int()%2 == 0
//	})
func DefaultValidator() *validator.Validate {
	if dv, ok := getValidator().(*defaultValidate); ok {
		return dv.inst
	}
	return nil
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

// Validate runs struct validation on dest using the configured Validator.
// Call this explicitly after BindJSON, BindXML, or BindForm.
func Validate(dest any) error {
	v := getValidator()
	if v == nil {
		return nil
	}
	return v.Validate(dest)
}
