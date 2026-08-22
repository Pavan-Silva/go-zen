package zen

import (
	"errors"
	"fmt"
	"mime/multipart"
	"net/http"
	"reflect"
)

// BindMultipleUnmarshaler is the interface implemented by types that can
// deserialize a slice of string values. When a struct field implements this
// interface the entire set of values for the matching form key is passed at
// once rather than being iterated element by element.
type BindMultipleUnmarshaler interface {
	UnmarshalParams(params []string) error
}

// Precomputed reflect types for multipart.FileHeader and common variants.
// Used by isFieldMultipartFile and setMultipartFileHeaderTypes to avoid
// repeated reflect.TypeOf calls on the hot path.
var (
	multipartFileHeaderType             = reflect.TypeFor[multipart.FileHeader]()
	multipartFileHeaderPointerType      = reflect.TypeFor[*multipart.FileHeader]()
	multipartFileHeaderSliceType        = reflect.TypeFor[[]multipart.FileHeader]()
	multipartFileHeaderPointerSliceType = reflect.TypeFor[[]*multipart.FileHeader]()
)

// multipartFormValues parses the request as a multipart form (using the
// given maxMemory) and returns the parsed form. Callers should use
// params.Value for text fields and params.File for file uploads.
func multipartFormValues(req *http.Request, maxMemory int64) (*multipart.Form, error) {
	if err := req.ParseMultipartForm(maxMemory); err != nil {
		return nil, fmt.Errorf("http: ParseMultipartForm error: %w", err)
	}
	return req.MultipartForm, nil
}

// isFieldMultipartFile reports whether field corresponds to one of the
// supported multipart.FileHeader types. An error is returned when the field
// is a bare multipart.FileHeader struct (use *multipart.FileHeader instead).
func isFieldMultipartFile(field reflect.Type) (bool, error) {
	switch field {
	case multipartFileHeaderPointerType,
		multipartFileHeaderSliceType,
		multipartFileHeaderPointerSliceType:
		return true, nil
	case multipartFileHeaderType:
		return true, errors.New("binding to multipart.FileHeader struct is not supported, use pointer to struct")
	default:
		return false, nil
	}
}

// setMultipartFileHeaderTypes populates structField with file headers from
// files that match inputFieldName. It returns true when the field was
// populated and false when no matching files were found, the target type is
// unsupported, or the first matched header is nil. Supported target types
// are *multipart.FileHeader, []multipart.FileHeader, and
// []*multipart.FileHeader.
func setMultipartFileHeaderTypes(
	structField reflect.Value,
	inputFieldName string,
	files map[string][]*multipart.FileHeader,
) bool {
	fileHeaders := files[inputFieldName]
	if len(fileHeaders) == 0 {
		return false
	}

	result := true
	switch structField.Type() {
	case multipartFileHeaderPointerSliceType:
		structField.Set(reflect.ValueOf(fileHeaders))
	case multipartFileHeaderSliceType:
		headers := make([]multipart.FileHeader, 0, len(fileHeaders))
		for _, fileHeader := range fileHeaders {
			if fileHeader == nil {
				continue
			}
			headers = append(headers, *fileHeader)
		}
		structField.Set(reflect.ValueOf(headers))
	case multipartFileHeaderPointerType:
		if fileHeaders[0] == nil {
			return false
		}
		structField.Set(reflect.ValueOf(fileHeaders[0]))
	default:
		result = false
	}

	return result
}
