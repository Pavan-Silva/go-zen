package zen

import (
	"io"
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
			if valueEnd == -1 {
				return rawQuery[valueStart:]
			}
			return rawQuery[valueStart : valueStart+valueEnd]
		}
	}
}

// Param returns the URL path parameter for the given key using Go 1.22+
// enhanced routing via [http.Request.PathValue].
//
// Given a route registered as "GET /users/{id}", the handler can retrieve
// the captured segment with:
//
//	id := c.Param("id")  // GET /users/42 -> "42"
func (c *Ctx) Param(key string) string {
	return c.Request.PathValue(key)
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
