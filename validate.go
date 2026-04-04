package zen

import (
	"net/mail"
	"reflect"
	"regexp"
	"strings"
	"sync"

	"github.com/go-playground/validator/v10"
)

// validator is lazily initialized on first use to avoid startup overhead.
// Using sync.Once ensures thread-safe initialization even with concurrent requests.
var (
	validateOnce sync.Once
	validateInst *validator.Validate
)

// emailRegex is a performant email validation pattern.
// It's compiled once at package init and reused.
var emailRegex = regexp.MustCompile(`^[a-zA-Z0-9._%+\-]+@[a-zA-Z0-9.\-]+\.[a-zA-Z]{2,}$`)

// validatorInstance returns a singleton validator instance, initializing it lazily.
// Custom validators ("email", "email-strict") are registered on first call.
// Tag names are mapped from "validate" struct tags, and field names are resolved
// from "json" tags for consistency with BindJSON/BindForm.
//
// This lazy initialization pattern means zero validator overhead if validation
// is not used in your application.
func validatorInstance() *validator.Validate {
	validateOnce.Do(func() {
		validateInst = validator.New()
		validateInst.SetTagName("validate")

		// Map field names from "json" tags so error messages use consistent names.
		validateInst.RegisterTagNameFunc(func(fld reflect.StructField) string {
			tag := fld.Tag.Get("json")
			if tag == "" || tag == "-" {
				return ""
			}
			// Strip tag options (e.g., "name,omitempty" → "name")
			if i := strings.IndexByte(tag, ','); i != -1 {
				tag = tag[:i]
			}
			return tag
		})

		// Register custom "email" validator: permissive but fast.
		if err := validateInst.RegisterValidation("email", func(fl validator.FieldLevel) bool {
			f := fl.Field()
			if f.Kind() != reflect.String {
				return false
			}
			return emailRegex.MatchString(f.String())
		}); err != nil {
			panic(err)
		}

		// Register custom "email-strict" validator: uses stdlib mail.ParseAddress.
		// This is slower but more accurate for RFC 5322 compliance.
		if err := validateInst.RegisterValidation("email-strict", func(fl validator.FieldLevel) bool {
			f := fl.Field()
			if f.Kind() != reflect.String {
				return false
			}
			_, err := mail.ParseAddress(f.String())
			return err == nil
		}); err != nil {
			panic(err)
		}
	})
	return validateInst
}
