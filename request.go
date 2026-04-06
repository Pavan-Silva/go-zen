package zen

import (
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"reflect"
)

// maxBodyBytes limits request body size to 1MB to prevent memory exhaustion attacks.
const maxBodyBytes = 1 << 20

// BindJSON parses the request body as JSON and decodes it into dest.
// If dest is a struct, it also runs struct validation using the registered validator.
// The request body is closed after reading (errors are logged but not returned).
//
// Returns an error if:
// - The JSON is malformed
// - dest is not a pointer to a struct for validation
// - Validation fails (if enabled)
//
// Example:
//
//	type SignupRequest struct {
//	    Email string `json:"email" validate:"required,email"`
//	    Age   int    `json:"age" validate:"required,gte=18"`
//	}
//	var req SignupRequest
//	if err := c.BindJSON(&req); err != nil {
//	    c.SendError(BadRequest(err.Error()))
//	    return
//	}
func (c *Context) BindJSON(dest any) error {
	lr := io.LimitReader(c.Request.Body, maxBodyBytes)
	var err error
	if err = json.NewDecoder(lr).Decode(dest); err != nil {
		closeErr := c.Request.Body.Close()
		if closeErr != nil {
			return fmt.Errorf("%w; body close error: %v", err, closeErr)
		}
		return err
	}

	if err = c.Request.Body.Close(); err != nil {
		return err
	}

	val := reflect.ValueOf(dest)
	if val.Kind() == reflect.Ptr {
		val = val.Elem()
	}
	if val.Kind() == reflect.Struct {
		return validatorInstance().Struct(dest)
	}
	return nil
}

// QueryParam returns the first value of a query parameter from the request URL.
// Query parameters are lazily parsed and cached on first access for performance.
// Returns an empty string if the parameter is not present.
//
// Example:
//
//	page := c.QueryParam("page")      // GET /posts?page=2 → "2"
//	search := c.QueryParam("q")       // GET /posts?q=golang → "golang"
//	missing := c.QueryParam("foo")    // GET /posts → ""
//
// Multiple values (e.g., "?item=1&item=2") return only the first value.
// Use c.Request.URL.Query() directly for access to all values.
func (c *Context) QueryParam(key string) string {
	if c.queryCache == nil {
		m := make(map[string]string)
		for k, vals := range c.Request.URL.Query() {
			if len(vals) > 0 {
				m[k] = vals[0]
			}
		}
		c.queryCache = m
	}
	return c.queryCache[key]
}

// Param returns the URL path parameter for the given key using Go 1.22+
// enhanced routing via [http.Request.PathValue].
//
// Given a route registered as "GET /users/{id}", the handler can retrieve
// the captured segment with:
//
//	id := c.Param("id")  // GET /users/42  →  "42"
//
// Returns an empty string if the parameter is not found. Requires Go 1.22+ router.
func (c *Context) Param(key string) string {
	return c.Request.PathValue(key)
}

// Body reads and returns the complete raw request body as a byte slice.
// It is the caller's responsibility to interpret the bytes (e.g. as plain
// text, XML, or a custom binary format).
//
// For JSON payloads prefer [Context.BindJSON], which streams directly and
// runs validation in the same call.
//
// The request body is closed after reading. Close errors are silently
// discarded.
//
// Example:
//
//	data, err := c.Body()
//	if err != nil {
//	    c.SendError(InternalError("failed to read body"))
//	    return
//	}
//	// Process raw bytes, e.g. XML parsing, custom format, etc.
func (c *Context) Body() ([]byte, error) {
	lr := io.LimitReader(c.Request.Body, maxBodyBytes)
	b, err := io.ReadAll(lr)
	closeErr := c.Request.Body.Close()
	if err != nil {
		if closeErr != nil {
			return nil, fmt.Errorf("%w; body close error: %v", err, closeErr)
		}
		return nil, err
	}
	if closeErr != nil {
		return nil, closeErr
	}
	return b, nil
}

// BindFile retrieves a single file from a multipart form upload by field name.
// It returns the file header and the file content as []byte.
//
// Example:
//
//	file, content, err := c.BindFile("avatar")
//	if err != nil {
//	    c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
//	    return
//	}
//	// file.Filename, file.Size, file.Header (e.g., Content-Type available)
func (c *Context) BindFile(fieldName string) (*multipart.FileHeader, []byte, error) {
	if err := c.Request.ParseMultipartForm(maxBodyBytes); err != nil {
		return nil, nil, fmt.Errorf("zen: ParseMultipartForm error: %w", err)
	}

	file, header, err := c.Request.FormFile(fieldName)
	if err != nil {
		return nil, nil, fmt.Errorf("zen: FormFile error: %w", err)
	}
	defer func() {
		if cerr := file.Close(); cerr != nil && err == nil {
			err = fmt.Errorf("zen: file close error: %w", cerr)
		}
	}()

	content, err := io.ReadAll(io.LimitReader(file, maxBodyBytes))
	if err != nil {
		return nil, nil, fmt.Errorf("zen: file read error: %w", err)
	}

	return header, content, nil
}

// BindFiles retrieves all files from a multipart form upload by field name.
// It returns a slice of file headers paired with their content.
//
// Example:
//
//	files, err := c.BindFiles("attachments")
//	if err != nil {
//	    c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
//	    return
//	}
//	for _, file := range files {
//	    fmt.Println(file.Header.Filename, len(file.Content))
//	}
func (c *Context) BindFiles(fieldName string) ([]UploadedFile, error) {
	if err := c.Request.ParseMultipartForm(maxBodyBytes); err != nil {
		return nil, fmt.Errorf("zen: ParseMultipartForm error: %w", err)
	}

	formFiles := c.Request.MultipartForm.File[fieldName]
	if len(formFiles) == 0 {
		return nil, fmt.Errorf("zen: no files found for field %q", fieldName)
	}

	var result []UploadedFile
	for _, header := range formFiles {
		file, err := header.Open()
		if err != nil {
			return nil, fmt.Errorf("zen: open file error: %w", err)
		}

		content, err := io.ReadAll(io.LimitReader(file, maxBodyBytes))
		closeErr := file.Close()
		if err != nil {
			return nil, fmt.Errorf("zen: file read error: %w", err)
		}
		if closeErr != nil {
			return nil, fmt.Errorf("zen: file close error: %w", closeErr)
		}

		result = append(result, UploadedFile{
			Header:  header,
			Content: content,
		})
	}

	return result, nil
}

// UploadedFile represents a file uploaded in a multipart request.
type UploadedFile struct {
	Header  *multipart.FileHeader
	Content []byte
}
