package zen

import (
	"fmt"
	"io"
	"mime/multipart"
)

// defaultMultipartMemory defines the maximum memory used when parsing multipart form data.
// Default is 32 MiB.
const defaultMultipartMemory int64 = 32 << 20

// parseMultipartForm ensures the multipart form is parsed.
func (c *Ctx) parseMultipartForm() error {
	if c.Request.MultipartForm == nil {
		maxMemory := c.engine.MaxMultipartMemory
		if maxMemory <= 0 {
			maxMemory = defaultMultipartMemory
		}

		if err := c.Request.ParseMultipartForm(maxMemory); err != nil {
			return fmt.Errorf("http: ParseMultipartForm error: %w", err)
		}
	}
	return nil
}

// FormFile retrieves a single file from a multipart form upload.
// Returns the file header and a file handle for streaming.
// The caller is responsible for closing the file.
//
// Example:
//
//	header, file, err := c.FormFile("avatar")
//	if err != nil {
//	    c.Error(http.StatusBadRequest, err.Error())
//	    return
//	}
//	defer file.Close()
func (c *Ctx) FormFile(fieldName string) (*multipart.FileHeader, multipart.File, error) {
	if err := c.parseMultipartForm(); err != nil {
		return nil, nil, err
	}

	file, header, err := c.Request.FormFile(fieldName)
	if err != nil {
		return nil, nil, fmt.Errorf("http: FormFile error: %w", err)
	}

	return header, file, nil
}

// FormFiles retrieves all file headers for a multipart form field.
// The caller opens each file individually for streaming access.
//
// Example:
//
//	headers, err := c.FormFiles("attachments")
//	if err != nil {
//	    c.Error(http.StatusBadRequest, err.Error())
//	    return
//	}
//	for _, h := range headers {
//	    file, _ := h.Open()
//	    defer file.Close()
//	}
func (c *Ctx) FormFiles(fieldName string) ([]*multipart.FileHeader, error) {
	if err := c.parseMultipartForm(); err != nil {
		return nil, err
	}

	formFiles := c.Request.MultipartForm.File[fieldName]
	if len(formFiles) == 0 {
		return nil, fmt.Errorf("http: no files found for field %q", fieldName)
	}

	return formFiles, nil
}

// ReadFile is a convenience method that reads a single uploaded file into memory.
// For large files, use FormFile() and stream the content instead.
func (c *Ctx) ReadFile(fieldName string) (header *multipart.FileHeader, content []byte, err error) {
	header, file, err := c.FormFile(fieldName)
	if err != nil {
		return nil, nil, err
	}

	// Named return parameter scope ensures clean defer error capture mechanics
	defer func() {
		if closeErr := file.Close(); closeErr != nil && err == nil {
			err = closeErr
		}
	}()

	content, err = io.ReadAll(file)
	if err != nil {
		return nil, nil, fmt.Errorf("http: file read error: %w", err)
	}

	return header, content, nil
}
