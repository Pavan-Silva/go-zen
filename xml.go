package zen

import (
	"bytes"
	"encoding/xml"
	"net/http"

	"github.com/Pavan-Silva/go-zen/logger"
)

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
