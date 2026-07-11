package zen

import (
	"io"
	"mime"
	"net"
	"net/url"
	"strings"
)

// QueryParam returns the first value of a query parameter from the request URL.
// Returns an empty string if the parameter is not present.
//
// Example:
//
//	page := c.QueryParam("page")      // GET /posts?page=2 -> "2"
//	search := c.QueryParam("q")       // GET /posts?q=golang-> "golang"
func (c *Ctx) QueryParam(key string) string {
	rawQuery := c.Request.URL.RawQuery
	if rawQuery == "" || key == "" {
		return ""
	}

	// Look for the key boundary configurations matching target bounds
	keyLen := len(key)
	pos := 0
	for {
		idx := strings.Index(rawQuery[pos:], key)
		if idx == -1 {
			return ""
		}
		start := pos + idx
		pos = start + keyLen

		// Ensure we matched the whole key, not just a substring suffix
		if (start == 0 || rawQuery[start-1] == '&') && (pos == len(rawQuery) || rawQuery[pos] == '=') {
			if pos == len(rawQuery) || rawQuery[pos] != '=' {
				return ""
			}

			valueStart := pos + 1
			valueEnd := strings.IndexByte(rawQuery[valueStart:], '&')

			var val string
			if valueEnd == -1 {
				val = rawQuery[valueStart:]
			} else {
				val = rawQuery[valueStart : valueStart+valueEnd]
			}
			if decoded, err := url.QueryUnescape(val); err == nil {
				return decoded
			}
			return val
		}
	}
}

// Param returns the URL path parameter for the given key.
//
// Given a route registered as "GET /users/{id}", the handler can retrieve
// the captured segment with:
//
//	id := c.Param("id")  // GET /users/42 -> "42"
func (c *Ctx) Param(key string) string {
	v, _ := c.ps.get(key)
	return v
}

// QueryParams returns all query parameters from the request URL as a map.
// Each key maps to a slice of values (e.g. ?a=1&a=2 -> {"a": ["1", "2"]}).
func (c *Ctx) QueryParams() map[string][]string {
	return c.Request.URL.Query()
}

// DefaultQuery returns the query parameter value for the given key, falling
// back to the provided default value if the parameter is missing or empty.
//
// Example:
//
//	c.DefaultQuery("page", "1")  // GET /items?page=3 -> "3"
//	c.DefaultQuery("page", "1")  // GET /items -> "1"
func (c *Ctx) DefaultQuery(key, defaultVal string) string {
	v := c.QueryParam(key)
	if v == "" {
		return defaultVal
	}
	return v
}

// DefaultParam returns the path parameter value for the given key, falling
// back to the provided default value if the parameter is missing or empty.
//
// Example:
//
//	c.DefaultParam("id", "default")  // GET /users/42 -> "42"
//	c.DefaultParam("id", "default")  // GET /users -> "default"
func (c *Ctx) DefaultParam(key, defaultVal string) string {
	v := c.Param(key)
	if v == "" {
		return defaultVal
	}
	return v
}

// Params returns all matched URL path parameters as a map.
func (c *Ctx) Params() map[string]string {
	m := make(map[string]string, len(c.ps))
	for _, p := range c.ps {
		m[p.Key] = p.Value
	}
	return m
}

// ClientIP returns the client's IP address by checking X-Forwarded-For and
// X-Real-IP headers before falling back to the remote address.
func (c *Ctx) ClientIP() string {
	if fwd := c.Request.Header["X-Forwarded-For"]; len(fwd) > 0 {
		if before, _, ok := strings.Cut(fwd[0], ","); ok {
			return strings.TrimSpace(before)
		}
		return strings.TrimSpace(fwd[0])
	}
	if realIP := c.Request.Header["X-Real-Ip"]; len(realIP) > 0 {
		return strings.TrimSpace(realIP[0])
	}
	host, _, err := net.SplitHostPort(c.Request.RemoteAddr)
	if err != nil {
		return c.Request.RemoteAddr
	}
	return host
}

// Body reads and returns the complete raw request body as a byte slice.
// It is the caller's responsibility to interpret the bytes.
func (c *Ctx) Body() ([]byte, error) {
	b, err := io.ReadAll(c.Request.Body)
	if err != nil {
		return nil, err
	}
	return b, nil
}

// Header returns the value of a request header by key.
// Returns an empty string if the header is not present.
//
// Example:
//
//	token := c.Header("Authorization")  // "Bearer <token>"
func (c *Ctx) Header(key string) string {
	return c.Request.Header.Get(key)
}

// IsAjax returns true if the request was made via XMLHttpRequest (X-Requested-With header).
func (c *Ctx) IsAjax() bool {
	return c.Request.Header.Get("X-Requested-With") == "XMLHttpRequest"
}

// ContentType returns the MIME content type of the request body, stripped of
// any parameters (e.g. "application/json", "text/html; charset=utf-8" → "text/html").
func (c *Ctx) ContentType() string {
	ct := c.Request.Header.Get("Content-Type")
	if ct == "" {
		return ""
	}
	t, _, _ := mime.ParseMediaType(ct)
	return t
}
