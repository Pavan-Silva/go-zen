package zen

import (
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"reflect"
)

const maxBodyBytes = 1 << 20

func (c *Context) BindJSON(dest any) error {
	lr := io.LimitReader(c.Request.Body, maxBodyBytes)
	err := json.NewDecoder(lr).Decode(dest)
	_ = c.Request.Body.Close()

	if err != nil {
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
// discarded for the same reason as in [Context.BindJSON].
func (c *Context) Body() ([]byte, error) {
	lr := io.LimitReader(c.Request.Body, maxBodyBytes)
	b, err := io.ReadAll(lr)
	_ = c.Request.Body.Close()
	return b, err
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
	defer file.Close()

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
		file.Close()
		if err != nil {
			return nil, fmt.Errorf("zen: file read error: %w", err)
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
