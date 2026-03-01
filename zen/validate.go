package zen

import (
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

		// Use the "binding" tag for validation rules, mirroring Gin.
		validateInst.SetTagName("binding")

		validateInst.RegisterTagNameFunc(func(fld reflect.StructField) string {
			tag := fld.Tag.Get("json")
			if tag == "" || tag == "-" {
				return ""
			}
			if i := strings.IndexByte(tag, ','); i != -1 {
				tag = tag[:i]
			}
			return tag
		})

		if err := validateInst.RegisterValidation("email", func(fl validator.FieldLevel) bool {
			f := fl.Field()
			if f.Kind() != reflect.String {
				return false
			}
			return emailRegex.MatchString(f.String())
		}); err != nil {
			panic(err)
		}

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
