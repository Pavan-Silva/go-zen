package zen

import (
	"fmt"
	"io"
	"mime/multipart"
)

// defaultMultipartMemory defines the maximum memory used when parsing multipart form data.
// Default is 32 MiB.
const defaultMultipartMemory int64 = 32 << 20

// UploadedFile represents a file uploaded in a multipart request.
type UploadedFile struct {
	Header  *multipart.FileHeader
	Content []byte
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
