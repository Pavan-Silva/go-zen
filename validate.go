package zen

import (
	"net/mail"
	"reflect"
	"regexp"
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
var defaultValidator Validator = new(defaultValidate)

// defaultValidate is the built-in validator using go-playground/validator/v10.
// Lazily initialized on first use.
type defaultValidate struct {
	once sync.Once
	inst *validator.Validate
}

func (v *defaultValidate) Validate(i any) error {
	v.once.Do(func() { v.inst = newValidator() })
	val := reflect.ValueOf(i)
	if val.Kind() == reflect.Pointer {
		val = val.Elem()
	}
	if val.Kind() != reflect.Struct {
		return nil
	}
	if !hasValidateTags(val.Type()) {
		return nil
	}
	return v.inst.Struct(i)
}

// hasValidateTags caches whether a struct type has any validate tags.
// After the one-time reflection scan (~43ns), subsequent lookups are a
// fast sync.Map load (~8ns, 0 allocs).
var hasValidateTagsCache sync.Map

func hasValidateTags(t reflect.Type) bool {
	if t.Kind() == reflect.Pointer {
		t = t.Elem()
	}
	if t.Kind() != reflect.Struct {
		return false
	}
	if v, ok := hasValidateTagsCache.Load(t); ok {
		return v.(bool)
	}
	for i := 0; i < t.NumField(); i++ {
		if t.Field(i).Tag.Get("validate") != "" {
			hasValidateTagsCache.Store(t, true)
			return true
		}
	}
	hasValidateTagsCache.Store(t, false)
	return false
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

	emailRegex := regexp.MustCompile(`^[a-zA-Z0-9._%+\-]+@[a-zA-Z0-9.\-]+\.[a-zA-Z]{2,}$`)

	if err := inst.RegisterValidation("email", func(fl validator.FieldLevel) bool {
		f := fl.Field()
		if f.Kind() != reflect.String {
			return false
		}
		return emailRegex.MatchString(f.String())
	}); err != nil {
		panic(err)
	}

	if err := inst.RegisterValidation("email-strict", func(fl validator.FieldLevel) bool {
		f := fl.Field()
		if f.Kind() != reflect.String {
			return false
		}
		_, err := mail.ParseAddress(f.String())
		return err == nil
	}); err != nil {
		panic(err)
	}

	return inst
}

// SetValidator sets a custom validator for request validation.
// Pass nil to disable validation entirely.
func SetValidator(v Validator) {
	defaultValidator = v
}

// Validate runs struct validation on dest using the configured Validator.
// BindJSON, BindXML, and BindForm call this automatically after decoding.
func Validate(dest any) error {
	if defaultValidator == nil {
		return nil
	}
	return defaultValidator.Validate(dest)
}
