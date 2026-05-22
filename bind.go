package zen

import (
	"fmt"
	"reflect"
	"strconv"
	"strings"
	"sync"
)

var bindMetaCache sync.Map // map[reflect.Type]*bindMeta

type bindFieldInfo struct {
	index    int
	kind     reflect.Kind
	paramTag string
	queryTag string
	formTag  string
}

type bindMeta struct {
	fields []bindFieldInfo
}

func getBindMeta(rt reflect.Type) *bindMeta {
	if meta, ok := bindMetaCache.Load(rt); ok {
		return meta.(*bindMeta)
	}
	meta := buildBindMeta(rt)
	actual, _ := bindMetaCache.LoadOrStore(rt, meta)
	return actual.(*bindMeta)
}

func buildBindMeta(rt reflect.Type) *bindMeta {
	fields := make([]bindFieldInfo, 0, rt.NumField())
	for i := 0; i < rt.NumField(); i++ {
		field := rt.Field(i)
		if field.PkgPath != "" {
			continue
		}

		fields = append(fields, bindFieldInfo{
			index:    i,
			kind:     field.Type.Kind(),
			paramTag: parseStructTag(field, "param"),
			queryTag: parseStructTag(field, "query"),
			formTag:  parseStructTag(field, "form"),
		})
	}
	return &bindMeta{fields: fields}
}

// BindBody binds only the request body to dest, ignoring path/query params.
// This matches Echo's c.BindBody() behavior.
//
// Returns an error if the content type is unsupported or binding fails.
//
// Example:
//
//	var req SignupRequest
//	if err := c.BindBody(&req); err != nil {
//	    c.Error(http.StatusBadRequest, err.Error())
//	    return
//	}
func (c *Context) BindBody(dest any) error {
	ct := c.Request.Header.Get("Content-Type")
	if ct == "" {
		return nil // No body to bind (e.g., GET with query params only)
	}
	before, _, _ := strings.Cut(ct, ";")
	ct = strings.ToLower(strings.TrimSpace(before))

	switch ct {
	case "application/json":
		return c.BindJSON(dest)
	case "application/xml", "text/xml":
		return c.BindXML(dest)
	case "application/x-www-form-urlencoded", "multipart/form-data":
		return c.BindForm(dest)
	default:
		if strings.HasSuffix(ct, "+json") || strings.HasSuffix(ct, "/json") {
			return c.BindJSON(dest)
		}
		return fmt.Errorf("BindBody: unsupported content type %q", ct)
	}
}

// Bind auto-detects the content type and binds the request body to dest.
// For GET/DELETE/HEAD requests, it also binds query parameters.
// This matches Echo's c.Bind() behavior.
//
// NOTE: This does NOT auto-validate (matching Gin/Echo behavior).
// Call Validate(dest) separately if you want struct validation.
//
// Example:
//
//	var req SignupRequest
//	if err := c.Bind(&req); err != nil {
//	    c.Error(http.StatusBadRequest, err.Error())
//	    return
//	}
//	if err := Validate(&req); err != nil {
//	    c.Error(http.StatusBadRequest, err.Error())
//	    return
//	}
func (c *Context) Bind(dest any) error {
	// Bind path parameters
	if err := c.BindPathParams(dest); err != nil {
		return err
	}

	// For GET/DELETE/HEAD, also bind query params (Echo behavior)
	method := c.Request.Method
	if method == "GET" || method == "DELETE" || method == "HEAD" {
		if err := c.BindQueryParams(dest); err != nil {
			return err
		}
	}

	// For requests with body, bind the body
	ct := c.Request.Header.Get("Content-Type")
	if ct != "" {
		return c.BindBody(dest)
	}
	return nil
}

// BindPathParams binds path parameters to dest struct fields.
// Field names are matched using "param" struct tags, falling back to "json" tags.
//
// This matches Echo's c.BindPathParams() behavior.
//
// Example:
//
//	var req ProfileRequest
//	if err := c.BindPathParams(&req); err != nil {
//	    c.Error(http.StatusBadRequest, err.Error())
//	    return
//	}
func (c *Context) BindPathParams(dest any) error {
	rv := reflect.ValueOf(dest)
	if rv.Kind() != reflect.Pointer || rv.Elem().Kind() != reflect.Struct {
		return nil // nothing to bind
	}

	rv = rv.Elem()
	rt := rv.Type()

	meta := getBindMeta(rt)
	for _, field := range meta.fields {
		val := c.Param(field.paramTag)
		if val == "" {
			continue
		}

		fv := rv.Field(field.index)
		if err := setFieldValue(fv, field.kind, []string{val}); err != nil {
			return fmt.Errorf("path param %q: %w", field.paramTag, err)
		}
	}
	return nil
}

// BindQueryParams binds query parameters to dest struct fields.
// Field names are matched using "query" struct tags, falling back to "json" tags.
//
// This matches Echo's c.BindQueryParams() behavior.
//
// Example:
//
//	var req SearchRequest
//	if err := c.BindQueryParams(&req); err != nil {
//	    c.Error(http.StatusBadRequest, err.Error())
//	    return
//	}
func (c *Context) BindQueryParams(dest any) error {
	rv := reflect.ValueOf(dest)
	if rv.Kind() != reflect.Pointer || rv.Elem().Kind() != reflect.Struct {
		return nil // nothing to bind
	}

	rv = rv.Elem()
	rt := rv.Type()

	query := c.Request.URL.Query()
	if len(query) == 0 {
		return nil
	}

	meta := getBindMeta(rt)
	for _, field := range meta.fields {
		vals, ok := query[field.queryTag]
		if !ok || len(vals) == 0 {
			continue
		}

		fv := rv.Field(field.index)
		if err := setFieldValue(fv, field.kind, vals); err != nil {
			return fmt.Errorf("query param %q: %w", field.queryTag, err)
		}
	}
	return nil
}

// parseStructTag extracts the field name from struct tag.
// It uses tagName first, falls back to "json", then the field name.
// Options after comma (e.g., ",omitempty") are stripped.
func parseStructTag(field reflect.StructField, tagName string) string {
	tag := field.Tag.Get(tagName)
	if tag == "" || tag == "-" {
		tag = field.Tag.Get("json")
	}
	if tag == "" || tag == "-" {
		tag = field.Name
	}
	if idx := strings.IndexByte(tag, ','); idx != -1 {
		tag = tag[:idx]
	}
	return tag
}

// setFieldValue converts string values to the target field type.
// This is shared with BindForm.
func setFieldValue(fv reflect.Value, kind reflect.Kind, vals []string) error {
	switch kind {
	case reflect.String:
		fv.SetString(vals[0])
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		n, err := strconv.ParseInt(vals[0], 10, 64)
		if err != nil {
			return err
		}
		fv.SetInt(n)
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		n, err := strconv.ParseUint(vals[0], 10, 64)
		if err != nil {
			return err
		}
		fv.SetUint(n)
	case reflect.Float32, reflect.Float64:
		f, err := strconv.ParseFloat(vals[0], 64)
		if err != nil {
			return err
		}
		fv.SetFloat(f)
	case reflect.Bool:
		switch vals[0] {
		case "true", "TRUE", "True", "1", "yes", "YES", "Yes", "on", "ON", "On":
			fv.SetBool(true)
		default:
			fv.SetBool(false)
		}
	case reflect.Slice:
		if fv.Type().Elem().Kind() == reflect.String {
			fv.Set(reflect.ValueOf(vals))
		}
	default:
	}
	return nil
}
