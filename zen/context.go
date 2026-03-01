package zen

import (
	"bytes"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"net/url"
	"reflect"
	"sync"
)

const maxPooledBufferSize = 64 * 1024 // 64 KB

var bufPool = sync.Pool{
	New: func() interface{} {
		return new(bytes.Buffer)
	},
}

type Context struct {
	Response   http.ResponseWriter
	Request    *http.Request
	queryCache url.Values
}

// JSON writes the given data as JSON to the response with the status code.
// Encoding is done before WriteHeader so that a failed encode does not commit
// a success status with an empty body. The buffer is capped before being
// returned to the pool so that one large response cannot permanently bloat it.
func (c *Context) JSON(status int, data interface{}) {
	buf := bufPool.Get().(*bytes.Buffer)
	buf.Reset()
	defer func() {
		if buf.Cap() <= maxPooledBufferSize {
			bufPool.Put(buf)
		}
	}()

	if err := json.NewEncoder(buf).Encode(data); err != nil {
		log.Printf("zen: JSON encode error: %v", err)
		return
	}

	h := c.Response.Header()
	if h.Get("Content-Type") == "" {
		h.Set("Content-Type", "application/json")
	}
	c.Response.WriteHeader(status)

	if _, err := buf.WriteTo(c.Response); err != nil {
		log.Printf("zen: JSON write error: %v", err)
	}
}

// BindJSON decodes a JSON request body into dest and validates it using
// binding struct tags. A single error is returned covering both decode and
// validation failures — the caller can use AsValidationErrors to distinguish
// between the two.
func (c *Context) BindJSON(dest interface{}) error {
	defer func() {
		if err := c.Request.Body.Close(); err != nil {
			log.Printf("zen: BindJSON body close error: %v", err)
		}
	}()

	buf := bufPool.Get().(*bytes.Buffer)
	buf.Reset()
	defer func() {
		if buf.Cap() <= maxPooledBufferSize {
			bufPool.Put(buf)
		}
	}()

	if _, err := buf.ReadFrom(c.Request.Body); err != nil {
		return err
	}
	if err := json.Unmarshal(buf.Bytes(), dest); err != nil {
		return err
	}

	// Skip validation for non structs
	val := reflect.ValueOf(dest)
	if val.Kind() == reflect.Ptr {
		val = val.Elem()
	}

	if val.Kind() == reflect.Struct {
		return validatorInstance().Struct(dest)
	}

	return nil
}

// FormValue retrieves a form value (works for both urlencoded forms and
// multipart). ParseForm is called at most once per request; subsequent calls
// reuse the already-populated Request.Form.
func (c *Context) FormValue(key string) string {
	if c.Request.Form == nil {
		if err := c.Request.ParseForm(); err != nil {
			log.Printf("zen: ParseForm error: %v", err)
		}
	}
	return c.Request.FormValue(key)
}

// QueryParam returns a URL query parameter, parsing the query string only
// once per request and caching the result. ParseQuery returns whatever it
// parsed before any error, so partial results are still useful and the error
// is intentionally ignored.
func (c *Context) QueryParam(key string) string {
	if c.queryCache == nil {
		c.queryCache, _ = url.ParseQuery(c.Request.URL.RawQuery)
	}
	return c.queryCache.Get(key)
}

// Param is a placeholder for named route parameters.
func (c *Context) Param(key string) string {
	return c.Request.PathValue(key)
}

// Body returns the raw request body as bytes.
// Body and BindJSON are mutually exclusive; calling both on the same request
// will result in the second call reading an already-closed body.
func (c *Context) Body() ([]byte, error) {
	defer func() {
		if err := c.Request.Body.Close(); err != nil {
			log.Printf("zen: Body close error: %v", err)
		}
	}()
	return io.ReadAll(c.Request.Body)
}
