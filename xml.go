package zen

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"reflect"

	"github.com/Pavan-Silva/go-zen/logger"
)

// BindXML parses the request body as XML and decodes it into dest.
// If dest is a struct, it also runs struct validation using the registered validator.
// The request body is closed after reading (errors are logged but not returned).
//
// Returns an error if:
// - The XML is malformed
// - dest is not a pointer to a struct for validation
// - Validation fails (if enabled)
//
// Example:
//
//	type SignupRequest struct {
//	    Email string `xml:"email"`
//	    Age   int    `xml:"age"`
//	}
//	var req SignupRequest
//	if err := c.BindXML(&req); err != nil {
//	    c.Error(http.StatusBadRequest, err.Error())
//	    return
//	}
func (c *Context) BindXML(dest any) error {
	dec := xml.NewDecoder(c.Request.Body)
	var err error
	if err = dec.Decode(dest); err != nil {
		closeErr := c.Request.Body.Close()
		if closeErr != nil {
			return fmt.Errorf("%w; body close error: %v", err, closeErr)
		}
		return err
	}

	// Ensure there is no trailing data after a single XML element.
	if err = dec.Decode(&struct{}{}); err != io.EOF {
		closeErr := c.Request.Body.Close()
		if closeErr != nil {
			return fmt.Errorf("request body must contain only one XML element; body close error: %v", closeErr)
		}
		if err == nil {
			return fmt.Errorf("request body must contain only one XML element")
		}
		return fmt.Errorf("request body must contain only one XML element: %w", err)
	}

	if err = c.Request.Body.Close(); err != nil {
		return err
	}

	val := reflect.ValueOf(dest)
	if val.Kind() == reflect.Pointer {
		val = val.Elem()
	}
	if val.Kind() == reflect.Struct {
		return validatorInstance().Struct(dest)
	}
	return nil
}

// XML encodes data as XML and writes it to the response with the given HTTP status.
// The response Content-Type header is automatically set to "application/xml".
// Uses a buffer pool to minimize allocations for each response.
//
// If encoding fails, logs the error and sends a 500 error response instead.
// Write errors are logged but not returned (they indicate connection issues).
//
// Example:
//
//	type User struct {
//	    XMLName xml.Name `xml:"user"`
//	    ID      int      `xml:"id"`
//	    Name    string   `xml:"name"`
//	}
//
//	c.XML(http.StatusOK, User{ID: 1, Name: "John"})
//
//	c.XML(http.StatusCreated, user)
func (c *Context) XML(status int, data any) {
	buf := responseBufPool.Get().(*bytes.Buffer)
	buf.Reset()

	if err := xml.NewEncoder(buf).Encode(data); err != nil {
		responseBufPool.Put(buf)
		logger.Error("HTTP: XML encode error: %v", err)
		http.Error(c.Response, "internal server error", http.StatusInternalServerError)
		return
	}

	c.Response.Header().Set("Content-Type", "application/xml")
	c.Response.WriteHeader(status)
	if _, err := c.Response.Write(buf.Bytes()); err != nil {
		logger.Error("HTTP: response write error: %v", err)
	}

	responseBufPool.Put(buf)
}
