package zen

import (
	"fmt"
	"strings"

	"google.golang.org/protobuf/proto"
)

// Bind auto-detects the content type and binds the request body to dest.
// It supports JSON, XML, and form data (URL-encoded and multipart).
// For form data, the request body is automatically parsed.
//
// Returns an error if the content type is unsupported or binding fails.
//
// Example:
//
//	var req SignupRequest
//	if err := c.Bind(&req); err != nil {
//	    c.Error(http.StatusBadRequest, err.Error())
//	    return
//	}
func (c *Context) Bind(dest any) error {
	ct := c.Request.Header.Get("Content-Type")
	ct = strings.ToLower(strings.TrimSpace(strings.Split(ct, ";")[0]))

	switch ct {
	case "application/json":
		return c.BindJSON(dest)
	case "application/xml", "text/xml":
		return c.BindXML(dest)
	case "application/x-protobuf":
		if pb, ok := dest.(proto.Message); ok {
			return c.BindProtoBuf(pb)
		}
		return fmt.Errorf("Bind: dest must implement proto.Message for protobuf content type")
	case "application/x-www-form-urlencoded":
		if err := c.Request.ParseForm(); err != nil {
			return fmt.Errorf("form parse: %w", err)
		}
		return c.BindForm(dest)
	case "multipart/form-data":
		if err := c.Request.ParseMultipartForm(32 << 20); err != nil { // 32 MB
			return fmt.Errorf("multipart parse: %w", err)
		}
		return c.BindForm(dest)
	default:
		// Try JSON as fallback (common case)
		if strings.Contains(ct, "json") {
			return c.BindJSON(dest)
		}
		return fmt.Errorf("Bind: unsupported content type %q", ct)
	}
}
