package zen

import (
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"reflect"
	"strconv"
	"strings"
	"sync"
)

// defaultMultipartMemory defines the maximum memory used when parsing multipart form data.
// Default is 32 MiB.
const defaultMultipartMemory int64 = 32 << 20

// UploadedFile represents a file uploaded in a multipart request.
type UploadedFile struct {
	Header  *multipart.FileHeader
	Content []byte
}

// ErrInvalidBindTarget is returned by BindForm when dest is not a pointer to a struct.
// This is a programming error (e.g., passing a bare struct instead of &struct).
// Using a sentinel error avoids allocation and allows exact comparison with errors.Is.
//
// Example:
//
//	var person Person  // Wrong: bare struct
//	if err := c.BindForm(person); err != nil {  // Will be ErrInvalidBindTarget
//	    ...
//	}
//
//	var person Person
//	if err := c.BindForm(&person); err != nil {  // Correct
//	    ...
//	}
var ErrInvalidBindTarget = errors.New("http: BindForm dest must be a pointer to a struct")

// FormError is returned by BindForm when a form value cannot be parsed into
// the target field's type (e.g., "abc" into an int field).
//
// Unlike generic errors, FormError provides structured access to the field name
// and underlying cause, allowing callers to generate precise error messages
// without string parsing.
//
// Example:
//
//	err := c.BindForm(&data)
//	if err != nil {
//	    var fe *zen.FormError
//	    if errors.As(err, &fe) {
//	        // Structured error: fe.Field and fe.Err
//	        c.JSON(http.StatusBadRequest, map[string]string{
//	            "field": fe.Field,
//	            "error": fe.Err.Error(),  // e.g., "value out of range"
//	        })
//	        return
//	    }
//	    // Handle other errors (validation, parsing, etc)
//	}
type FormError struct {
	// Field is the form key that failed (resolved from "json" tag or field name).
	Field string

	// Err is the underlying conversion error from strconv (e.g., strconv.ErrRange).
	Err error
}

// fieldInfo caches reflection metadata needed to bind form fields to structs.
// By pre-computing this once per type, we avoid expensive reflection on every request.
// The setFunc closure is pre-specialized to each field's type, so the type switch
// happens once at setup time, not on every field assignment.
type fieldInfo struct {
	// index is the struct field index for reflect.Value.Field(i)
	index int
	// kind is the target field's reflect.Kind (pre-computed for zero-copy type switches)
	kind reflect.Kind
	// formKey is the form parameter name (from struct tag or field name)
	formKey string
	// setFunc is a specialized closure for converting strings to the field type
	setFunc func(reflect.Value, string) error
}

// formCache stores pre-computed field metadata keyed by reflect.Type.
// Using sync.Map provides lock-free reads, which is critical in high-concurrency servers.
// The cache is lazily populated on first access per type, then reused forever.
var formCache sync.Map

// getFormFields returns cached field metadata for a struct type.
// On first call, it reflects on the struct, resolves field names from tags,
// and builds type-specialized setFunc closures. Subsequent calls return the cache hit.
//
// This is the key to zen's form binding performance: reflection happens once per type,
// not once per request.
func getFormFields(t reflect.Type) []fieldInfo {
	if cached, ok := formCache.Load(t); ok {
		return cached.([]fieldInfo)
	}

	numField := t.NumField()
	fields := make([]fieldInfo, 0, numField)

	for i := range numField {
		field := t.Field(i)

		// Skip unexported fields (PkgPath != "") and embedded fields (Anonymous).
		if !field.Anonymous && field.PkgPath == "" {
			tag := field.Tag.Get("json")
			// Strip tag options (e.g., "name,omitempty" → "name")
			if idx := strings.IndexByte(tag, ','); idx != -1 {
				tag = tag[:idx]
			}
			// Use field name if no tag or tag is "-"
			if tag == "" || tag == "-" {
				tag = field.Name
			}

			fields = append(fields, fieldInfo{
				index:   i,
				kind:    field.Type.Kind(),
				formKey: tag,
				setFunc: getSetFunc(field.Type.Kind()),
			})
		}
	}

	formCache.Store(t, fields)
	return fields
}

// getSetFunc returns a type-specialized closure for string-to-field conversion.
// The type switch happens here (once per field type), so the returned closure
// is nearly branch-free on the hot path. This is significantly faster than
// doing the type switch for every field on every request.
func getSetFunc(kind reflect.Kind) func(reflect.Value, string) error {
	return func(fv reflect.Value, raw string) error {
		switch kind {
		case reflect.String:
			fv.SetString(raw)

		case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
			n, err := strconv.ParseInt(raw, 10, 64)
			if err != nil {
				return err
			}
			fv.SetInt(n)

		case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
			n, err := strconv.ParseUint(raw, 10, 64)
			if err != nil {
				return err
			}
			fv.SetUint(n)

		case reflect.Float32, reflect.Float64:
			f, err := strconv.ParseFloat(raw, 64)
			if err != nil {
				return err
			}
			fv.SetFloat(f)

		case reflect.Bool:
			// Direct string comparison avoids strings.ToLower allocation and is faster.
			// Common true values are checked; anything else (including "") is false.
			switch raw {
			case "true", "TRUE", "True", "1", "yes", "YES", "Yes", "on", "ON", "On":
				fv.SetBool(true)
			default:
				fv.SetBool(false)
			}

		default:
			// Unsupported types are silently ignored (field keeps zero value).
			// This is consistent with the "unknown keys are ignored" behavior.
		}
		return nil
	}
}

// BindForm parses URL-encoded or multipart form data into a struct, then validates it.
// It's the form equivalent of BindJSON, using the same "json" struct tag for field mapping.
//
// This allows using a single struct for both JSON endpoints and form endpoints:
//
//	type UserData struct {
//	    Name  string `json:"name" validate:"required"`
//	    Email string `json:"email" validate:"required,email"`
//	    Age   int    `json:"age" validate:"gte=0,lte=200"`
//	}
//
//	// POST /api/users (JSON): c.BindJSON(&user)
//	// POST /register (form): c.BindForm(&user)
//
// Supported field types: string, bool, all int variants, all uint variants, float32, float64.
// Unsupported types are silently ignored (keep zero value).
//
// Returns errors:
// - ErrInvalidBindTarget: dest is not a pointer to struct
// - FormError: a field value failed to parse (includes field name and cause)
// - validator errors: validation failed
//
// Example:
//
//	type LoginForm struct {
//	    Email    string `json:"email" validate:"required,email"`
//	    Password string `json:"password" validate:"required,min=8"`
//	}
//	var form LoginForm
//	if err := c.BindForm(&form); err != nil {
//	    if fe := (*zen.FormError)(nil); errors.As(err, &fe) {
//	        c.JSON(http.StatusBadRequest, map[string]string{
//	            "field": fe.Field,
//	            "error": fe.Err.Error(),
//	        })
//	    }
//	    return
//	}
func (c *Context) BindForm(dest any) error {
	if err := c.Request.ParseForm(); err != nil {
		return fmt.Errorf("http: ParseForm error: %w", err)
	}

	if err := mapFormValues(c.Request.Form, dest); err != nil {
		return err
	}

	rv := reflect.ValueOf(dest)
	if rv.Kind() == reflect.Ptr {
		rv = rv.Elem()
	}
	if rv.Kind() == reflect.Struct {
		return validatorInstance().Struct(dest)
	}
	return nil
}

// mapFormValues maps form parameters onto struct fields using cached metadata.
// It's the core of form binding: iterates over cached fields and applies conversions.
//
// Returns ErrInvalidBindTarget if dest is not a pointer to struct, or FormError
// if a field value fails to convert.
func mapFormValues(values map[string][]string, dest any) error {
	rv := reflect.ValueOf(dest)
	if rv.Kind() != reflect.Pointer || rv.Elem().Kind() != reflect.Struct {
		return ErrInvalidBindTarget
	}

	rv = rv.Elem()
	fields := getFormFields(rv.Type())

	for _, fi := range fields {
		vals, ok := values[fi.formKey]
		if !ok || len(vals) == 0 {
			continue
		}

		if err := fi.setFunc(rv.Field(fi.index), vals[0]); err != nil {
			return &FormError{Field: fi.formKey, Err: err}
		}
	}
	return nil
}

// FormFile retrieves a single file from a multipart form upload by field name.
// It returns the file header and the file content as []byte.
//
// Example:
//
//	file, content, err := c.FormFile("avatar")
//	if err != nil {
//	    c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
//	    return
//	}
//	// file.Filename, file.Size, file.Header (e.g., Content-Type available)
func (c *Context) FormFile(fieldName string) (*multipart.FileHeader, []byte, error) {
	if err := c.Request.ParseMultipartForm(defaultMultipartMemory); err != nil {
		return nil, nil, fmt.Errorf("http: ParseMultipartForm error: %w", err)
	}

	file, header, err := c.Request.FormFile(fieldName)
	if err != nil {
		return nil, nil, fmt.Errorf("http: FormFile error: %w", err)
	}
	defer func() {
		if cerr := file.Close(); cerr != nil && err == nil {
			err = fmt.Errorf("http: file close error: %w", cerr)
		}
	}()

	content, err := io.ReadAll(file)
	if err != nil {
		return nil, nil, fmt.Errorf("http: file read error: %w", err)
	}

	return header, content, nil
}

// FormFiles retrieves all files from a multipart form upload by field name.
// It returns a slice of file headers paired with their content.
//
// Example:
//
//	files, err := c.FormFiles("attachments")
//	if err != nil {
//	    c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
//	    return
//	}
//	for _, file := range files {
//	    fmt.Println(file.Header.Filename, len(file.Content))
//	}
func (c *Context) FormFiles(fieldName string) ([]UploadedFile, error) {
	if err := c.Request.ParseMultipartForm(defaultMultipartMemory); err != nil {
		return nil, fmt.Errorf("http: ParseMultipartForm error: %w", err)
	}

	formFiles := c.Request.MultipartForm.File[fieldName]
	if len(formFiles) == 0 {
		return nil, fmt.Errorf("http: no files found for field %q", fieldName)
	}

	var result []UploadedFile
	for _, header := range formFiles {
		file, err := header.Open()
		if err != nil {
			return nil, fmt.Errorf("http: open file error: %w", err)
		}

		content, err := io.ReadAll(file)
		closeErr := file.Close()
		if err != nil {
			return nil, fmt.Errorf("http: file read error: %w", err)
		}
		if closeErr != nil {
			return nil, fmt.Errorf("http: file close error: %w", closeErr)
		}

		result = append(result, UploadedFile{
			Header:  header,
			Content: content,
		})
	}

	return result, nil
}

// Error implements the error interface.
func (e *FormError) Error() string {
	return "BindForm field \"" + e.Field + "\": " + e.Err.Error()
}

// Unwrap returns the underlying cause error, enabling errors.Is/As to traverse
// the error chain. This allows callers to match specific strconv errors:
//
//	if errors.Is(err, strconv.ErrRange) {
//	    // Field value was out of range for its type
//	}
func (e *FormError) Unwrap() error { return e.Err }
