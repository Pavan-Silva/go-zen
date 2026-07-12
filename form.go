package zen

import (
	"fmt"
	"maps"
	"net/http"
)

// BindForm binds form data from the request body to dest. It parses the
// request body as application/x-www-form-urlencoded (or multipart form data
// that was already parsed) and maps values onto struct fields tagged with
// the "form" struct tag.
func (c *Ctx) BindForm(dest any) error {
	params, err := formValues(c.Request)
	if err != nil {
		return err
	}
	if err := bindData(dest, params, "form", nil); err != nil {
		return err
	}
	if c.engine.autoValidate && c.engine.validator != nil {
		return c.engine.validator.Validate(dest)
	}
	return nil
}

// formValues parses the request form and returns the values as a
// map[string][]string. It delegates to net/http's ParseForm and copies the
// result into a fresh map.
func formValues(req *http.Request) (map[string][]string, error) {
	if err := req.ParseForm(); err != nil {
		return nil, fmt.Errorf("http: ParseForm error: %w", err)
	}
	m := make(map[string][]string, len(req.Form))
	maps.Copy(m, req.Form)
	return m, nil
}
