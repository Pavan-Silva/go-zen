package zen

import (
	"errors"
	"net/mail"
	"reflect"
	"regexp"
	"strings"
	"sync"

	"github.com/go-playground/validator/v10"
)

var (
	validateOnce sync.Once
	validateInst *validator.Validate
)

var emailRegex = regexp.MustCompile(`^[a-zA-Z0-9._%+\-]+@[a-zA-Z0-9.\-]+\.[a-zA-Z]{2,}$`)

func validatorInstance() *validator.Validate {
	validateOnce.Do(func() {
		validateInst = validator.New()
		validateInst.RegisterTagNameFunc(func(fld reflect.StructField) string {
			tag := fld.Tag.Get("json")
			if tag == "" || tag == "-" {
				return ""
			}
			// strings.SplitN allocates a slice on every call.
			// IndexByte finds the comma in one pass with zero allocations.
			if i := strings.IndexByte(tag, ','); i != -1 {
				tag = tag[:i]
			}
			return tag
		})

		errMail := validateInst.RegisterValidation("email", func(fl validator.FieldLevel) bool {
			field := fl.Field()
			if field.Kind() != reflect.String {
				return false
			}
			return emailRegex.MatchString(field.String())
		})
		if errMail != nil {
			panic(errMail)
		}

		errMailStrict := validateInst.RegisterValidation("email-strict", func(fl validator.FieldLevel) bool {
			field := fl.Field()
			if field.Kind() != reflect.String {
				return false
			}
			_, err := mail.ParseAddress(field.String())
			return err == nil
		})
		if errMailStrict != nil {
			panic(errMailStrict)
		}
	})
	return validateInst
}

// Validate runs struct validation and returns the raw error.
// Use ValidateErr if you need to return field-level messages to a client.
func (c *Context) Validate(v interface{}) error {
	return validatorInstance().Struct(v)
}

// ValidationError holds a single field-level validation failure.
type ValidationError struct {
	Field string `json:"field"`
	Tag   string `json:"tag"`
	Param string `json:"param,omitempty"`
}

// ValidateErr runs validation and returns a flat []ValidationError slice,
// bypassing the reflection-heavy .Error() string chain on the error path.
// Panics if v is not a pointer to a struct — that is always a programmer mistake.
// Returns nil when validation passes.
func (c *Context) ValidateErr(v interface{}) []ValidationError {
	err := validatorInstance().Struct(v)
	if err == nil {
		return nil
	}

	var ve validator.ValidationErrors
	if !errors.As(err, &ve) {
		panic("zen: ValidateErr called with non-struct value")
	}

	out := make([]ValidationError, len(ve))
	for i, fe := range ve {
		out[i] = ValidationError{
			Field: fe.Field(),
			Tag:   fe.Tag(),
			Param: fe.Param(),
		}
	}
	return out
}