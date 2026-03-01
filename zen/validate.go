package zen

import (
	"net/mail"
	"reflect"
	"regexp"
	"strings"
	"sync"

	"github.com/go-playground/validator/v10"
)

// validator instance reused across calls.  Using sync.Once so we don't
// re-create it on every request; the validator is safe for concurrent use.
var (
	validateOnce sync.Once
	validateInst *validator.Validate
)

var emailRegex = regexp.MustCompile(`^[a-zA-Z0-9._%+\-]+@[a-zA-Z0-9.\-]+\.[a-zA-Z]{2,}$`)

// validator.go
func validatorInstance() *validator.Validate {
	validateOnce.Do(func() {
		validateInst = validator.New()
		validateInst.RegisterTagNameFunc(func(fld reflect.StructField) string {
			name := strings.SplitN(fld.Tag.Get("json"), ",", 2)[0]
			if name == "-" {
				return ""
			}
			return name
		})
		// replace net/mail based email validator with simple regex
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

		// Use default mail validator (optional)
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

// Validate runs struct validation using `go-playground/validator` tags. It
// returns the error from the underlying library (which may be a
// validator.ValidationErrors slice) or nil if validation passes.
//
// The function is intentionally tiny and allocates only on error paths; the
// validator object itself is cached to avoid repeated initialization cost. This
// keeps the overhead near zero when validation isn't used.
func (c *Context) Validate(v interface{}) error {
	return validatorInstance().Struct(v)
}
