package zen

import (
	"bytes"
	"fmt"
	"io"
	"net/http"

	"gopkg.in/yaml.v3"
)

// YAML binds the request body as YAML into the provided interface.
// It reads the body directly from the request.
//
// Example:
//
//	var data MyStruct
//	if err := c.YAML(&data); err != nil {
//	    c.Error(http.StatusBadRequest, "invalid yaml")
//	    return
//	}
func (c *Context) BindYAML(dest any) error {
	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		return err
	}
	defer c.Request.Body.Close()

	if err := yaml.Unmarshal(body, dest); err != nil {
		return fmt.Errorf("yaml: %w", err)
	}

	return validateStruct(dest)
}

// MustBindYAML binds YAML and panics on error. Use only when you're sure the binding will succeed.
func (c *Context) MustBindYAML(dest any) {
	if err := c.BindYAML(dest); err != nil {
		panic(err)
	}
}

// YAML sends a YAML response with the given status code.
// Uses a buffer pool for efficient encoding.
//
// Example:
//
//	c.YAML(http.StatusOK, map[string]any{"status": "ok"})
func (c *Context) YAML(status int, data any) {
	buf := responseBufPool.Get().(*bytes.Buffer)
	buf.Reset()

	if err := yaml.NewEncoder(buf).Encode(data); err != nil {
		responseBufPool.Put(buf)
		http.Error(c.Response, "yaml encode error", http.StatusInternalServerError)
		return
	}

	c.Response.Header().Set("Content-Type", "application/yaml")
	c.Response.WriteHeader(status)
	c.Response.Write(buf.Bytes())
	responseBufPool.Put(buf)
}
