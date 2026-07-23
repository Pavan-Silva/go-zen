// Package zen provides binding of HTTP request data into Go structs and maps.
//
// # Subsystem overview
//
// The binding subsystem maps incoming data from four sources onto a destination:
//   - Path parameters  (via BindPathValues)
//   - Query parameters (via BindQueryParams)
//   - Request body     (via BindBody)
//   - HTTP headers     (via BindHeaders)
//
// # Tag resolution order
//
// For a given source each struct field is located by looking up its struct tag
// (param / query / form / header). If that tag is empty the json tag is tried
// next. If the json tag is also empty (or set to "-") the field name is used
// as-is. This allows a single set of json tags to double as binding tags in
// most cases.
//
// # Supported field types
//
// Basic types (int*, uint*, float*, bool, string), pointers to those types,
// slices of those types, time.Time (with format tag), BindUnmarshaler,
// encoding.TextUnmarshaler, and multipart.FileHeader variants.
package zen

import (
	"errors"
	"fmt"
	"mime/multipart"
	"net/http"
	"strings"
)

// ErrInvalidBindTarget is returned when the bind destination is not a pointer
// to a struct or map.
var ErrInvalidBindTarget = errors.New("http: bind dest must be a pointer to a struct")

// FormError represents a form binding error for a specific field.
type FormError struct {
	Field string
	Err   error
}

func (e *FormError) Error() string {
	return "form field \"" + e.Field + "\": " + e.Err.Error()
}

func (e *FormError) Unwrap() error { return e.Err }

// BindUnmarshaler is the interface used to wrap the UnmarshalParam method.
// Types implementing this interface gain control over how a single string
// value is deserialised during binding.
type BindUnmarshaler interface {
	UnmarshalParam(param string) error
}

// Bind binds path params, query params (GET/DELETE/HEAD), and the request
// body to dest. The order of precedence is:
//  1. Path parameters
//  2. Query parameters (only for GET, DELETE, HEAD)
//  3. Request body (Content-Type driven)
func (c *Ctx) Bind(dest any) error {
	if dest == nil {
		return ErrInvalidBindTarget
	}
	if err := bindPathValues(c, dest); err != nil {
		return err
	}
	method := c.Request.Method
	if method == http.MethodGet || method == http.MethodDelete || method == http.MethodHead {
		if err := bindQueryParams(c, dest); err != nil {
			return err
		}
	}
	if err := bindBody(c, dest); err != nil {
		return err
	}
	if c.engine.autoValidate && c.engine.validator != nil {
		return c.engine.validator.Validate(dest)
	}
	return nil
}

// BindPathValues binds URL path parameters to dest. Path params are
// extracted from the route pattern and mapped onto struct fields tagged
// with the "param" struct tag.
func BindPathValues(c *Ctx, dest any) error {
	if err := bindPathValues(c, dest); err != nil {
		return err
	}
	if c.engine.autoValidate && c.engine.validator != nil {
		return c.engine.validator.Validate(dest)
	}
	return nil
}

func bindPathValues(c *Ctx, dest any) error {
	params := map[string][]string{}
	for _, p := range c.ps {
		params[p.Key] = []string{p.Value}
	}
	if len(params) == 0 {
		return nil
	}
	return bindData(dest, params, "param", nil)
}

// BindQueryParams binds query parameters to dest. Query params are mapped
// onto struct fields tagged with the "query" struct tag.
func BindQueryParams(c *Ctx, dest any) error {
	if err := bindQueryParams(c, dest); err != nil {
		return err
	}
	if c.engine.autoValidate && c.engine.validator != nil {
		return c.engine.validator.Validate(dest)
	}
	return nil
}

func bindQueryParams(c *Ctx, dest any) error {
	if c.Request.URL.RawQuery == "" {
		return nil
	}
	return bindData(dest, c.Request.URL.Query(), "query", nil)
}

// BindBody binds the request body to dest based on the Content-Type header.
// Supported content types:
//   - application/json
//   - application/xml, text/xml
//   - application/x-www-form-urlencoded
//   - multipart/form-data
//   - any type ending with +json or /json
func BindBody(c *Ctx, dest any) error {
	if err := bindBody(c, dest); err != nil {
		return err
	}
	if c.engine.autoValidate && c.engine.validator != nil {
		return c.engine.validator.Validate(dest)
	}
	return nil
}

func bindBody(c *Ctx, dest any) error {
	var err error
	req := c.Request

	base, _, _ := strings.Cut(req.Header.Get("Content-Type"), ";")
	mediatype := strings.TrimSpace(base)
	if mediatype == "" {
		return nil
	}

	switch mediatype {
	case "application/json":
		err = c.engine.JSONSerializer.Deserialize(c, dest)
	case "application/xml", "text/xml":
		err = c.engine.XMLSerializer.Deserialize(c, dest)
	case "application/x-www-form-urlencoded":
		var params map[string][]string
		params, err = formValues(req)
		if err == nil {
			err = bindData(dest, params, "form", nil)
		}
	case "multipart/form-data":
		var params *multipart.Form
		params, err = multipartFormValues(req, c.engine.MaxMultipartMemory)
		if err == nil {
			err = bindData(dest, params.Value, "form", params.File)
		}
	default:
		if strings.HasSuffix(mediatype, "+json") || strings.HasSuffix(mediatype, "/json") {
			err = c.engine.JSONSerializer.Deserialize(c, dest)
		} else {
			return fmt.Errorf("BindBody: unsupported content type %q", mediatype)
		}
	}
	return err
}

// BindHeaders binds HTTP request headers to dest. Headers are mapped onto
// struct fields tagged with the "header" struct tag. Header names are
// matched case-insensitively.
func BindHeaders(c *Ctx, dest any) error {
	if err := bindHeaders(c, dest); err != nil {
		return err
	}
	if c.engine.autoValidate && c.engine.validator != nil {
		return c.engine.validator.Validate(dest)
	}
	return nil
}

func bindHeaders(c *Ctx, dest any) error {
	return bindData(dest, c.Request.Header, "header", nil)
}
