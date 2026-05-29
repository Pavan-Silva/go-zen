package zen

import (
	"fmt"
	"net/http"
	"reflect"
	"strconv"
	"strings"
	"sync"
)

// Global thread-safe metadata cache for multi-source data binding.
var bindMetaCache sync.Map

// bindFieldInfo tracks pre-compiled field metadata and memory mapping layouts.
type bindFieldInfo struct {
	index     int
	kind      reflect.Kind
	paramTag  string
	queryTag  string
	formTag   string
	headerTag string
}

// bindMeta holds the optimized slice of field plans for a unique type configuration.
type bindMeta struct {
	fields []bindFieldInfo
}

// getBindMeta fetches a compiled struct plan from the cache or creates it on a miss.
func getBindMeta(rt reflect.Type) *bindMeta {
	if meta, ok := bindMetaCache.Load(rt); ok {
		return meta.(*bindMeta)
	}
	meta := buildBindMeta(rt)
	actual, _ := bindMetaCache.LoadOrStore(rt, meta)
	return actual.(*bindMeta)
}

// buildBindMeta parses struct fields and tag configurations exactly once.
func buildBindMeta(rt reflect.Type) *bindMeta {
	numFields := rt.NumField()
	fields := make([]bindFieldInfo, 0, numFields)

	for i := range numFields {
		field := rt.Field(i)
		if field.PkgPath != "" && !field.Anonymous {
			continue
		}

		fields = append(fields, bindFieldInfo{
			index:     i,
			kind:      field.Type.Kind(),
			paramTag:  parseStructTag(field, "param"),
			queryTag:  parseStructTag(field, "query"),
			formTag:   parseStructTag(field, "form"),
			headerTag: parseHeaderTag(field),
		})
	}
	return &bindMeta{fields: fields}
}

// Bind auto-detects the request content type and decodes the payload into dest.
// For GET, DELETE, and HEAD requests, it automatically populates query parameters first.
func (c *Ctx) Bind(dest any) error {
	if err := c.BindPathParams(dest); err != nil {
		return err
	}

	method := c.Request.Method
	if method == "GET" || method == "DELETE" || method == "HEAD" {
		if err := c.BindQueryParams(dest); err != nil {
			return err
		}
	}

	if _, exists := c.Request.Header["Content-Type"]; exists {
		return c.BindBody(dest)
	}
	return nil
}

// BindBody parses the request body into dest based on the Content-Type header,
// completely skipping path and query parameters.
func (c *Ctx) BindBody(dest any) error {
	ct := ""
	if vals := c.Request.Header["Content-Type"]; len(vals) > 0 {
		ct = vals[0]
	}
	if ct == "" {
		return nil
	}

	// Fast-track extraction of mime-type boundary text
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

// BindPathParams extracts route wildcards using native Go 1.22+ request context maps.
func (c *Ctx) BindPathParams(dest any) error {
	rv := reflect.ValueOf(dest)
	if rv.Kind() != reflect.Pointer || rv.Elem().Kind() != reflect.Struct {
		return nil
	}

	rv = rv.Elem()
	rt := rv.Type()

	meta := getBindMeta(rt)
	for i := 0; i < len(meta.fields); i++ {
		field := meta.fields[i]
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

// BindQueryParams parses query parameters into dest struct fields.
func (c *Ctx) BindQueryParams(dest any) error {
	rv := reflect.ValueOf(dest)
	if rv.Kind() != reflect.Pointer || rv.Elem().Kind() != reflect.Struct {
		return nil
	}

	if c.Request.URL.RawQuery == "" {
		return nil
	}

	rv = rv.Elem()
	rt := rv.Type()

	meta := getBindMeta(rt)
	for i := 0; i < len(meta.fields); i++ {
		field := meta.fields[i]

		// Fast Path: Zero map allocations, parses offsets directly from the raw string line
		val := c.QueryParam(field.queryTag)
		if val == "" {
			continue
		}

		fv := rv.Field(field.index)
		if err := setFieldValue(fv, field.kind, []string{val}); err != nil {
			return fmt.Errorf("query param %q: %w", field.queryTag, err)
		}
	}
	return nil
}

// parseStructTag isolates clean lookup strings from structured field tag entries.
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

// setFieldValue safely converts raw string slices into the concrete target field types.
func setFieldValue(fv reflect.Value, kind reflect.Kind, vals []string) error {
	val := vals[0]

	switch kind {
	case reflect.String:
		fv.SetString(val)
	case reflect.Slice:
		// Specialized support for string slices ([]string)
		if fv.Type().Elem().Kind() == reflect.String {
			fv.Set(reflect.ValueOf(vals))
		}
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		n, err := strconv.ParseInt(val, 10, 64)
		if err != nil {
			return err
		}
		fv.SetInt(n)
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		n, err := strconv.ParseUint(val, 10, 64)
		if err != nil {
			return err
		}
		fv.SetUint(n)
	case reflect.Float32, reflect.Float64:
		n, err := strconv.ParseFloat(val, 64)
		if err != nil {
			return err
		}
		fv.SetFloat(n)
	case reflect.Bool:
		b, err := strconv.ParseBool(val)
		if err != nil {
			return err
		}
		fv.SetBool(b)
	default:
	}
	return nil
}

// parseHeaderTag parses the header tag with canonicalized field name fallback.
func parseHeaderTag(field reflect.StructField) string {
	tag := field.Tag.Get("header")
	if tag == "" || tag == "-" {
		tag = field.Tag.Get("json")
	}
	if tag == "" || tag == "-" {
		return http.CanonicalHeaderKey(field.Name)
	}
	if idx := strings.IndexByte(tag, ','); idx != -1 {
		tag = tag[:idx]
	}
	return tag
}

// BindHeader binds HTTP request headers to a struct using `header` or `json` tags.
// Header names are canonicalized via http.CanonicalHeaderKey at compile time.
//
// Example:
//
//	type Headers struct {
//	    UserID string `header:"X-User-Id"`
//	    APIKey string `header:"X-Api-Key"`
//	    Rate   int    `header:"X-Rate-Limit"`
//	}
func (c *Ctx) BindHeader(dest any) error {
	v := reflect.ValueOf(dest)
	if v.Kind() != reflect.Pointer || v.Elem().Kind() != reflect.Struct {
		return fmt.Errorf("header: dest must be a pointer to struct")
	}

	v = v.Elem()
	t := v.Type()
	meta := getBindMeta(t)

	for i := 0; i < len(meta.fields); i++ {
		f := meta.fields[i]
		if f.headerTag == "" {
			continue
		}

		headerValue := c.Request.Header.Get(f.headerTag)
		if headerValue == "" {
			continue
		}

		fv := v.Field(f.index)
		if err := setFieldValue(fv, f.kind, []string{headerValue}); err != nil {
			return fmt.Errorf("header %q: %w", f.headerTag, err)
		}
	}

	return nil
}
