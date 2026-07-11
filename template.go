package zen

import (
	"html/template"
	"io"
	"io/fs"
	"path/filepath"

	"github.com/Pavan-Silva/go-zen/logger"
)

// Templates holds parsed HTML templates for rendering.
type Templates struct {
	tmpl *template.Template
}

// LoadTemplates parses all templates matching the glob pattern from the given
// filesystem. Use os.DirFS for the local filesystem or embed.FS for embedded
// templates.
//
// Example:
//
//	t := zen.LoadTemplates(os.DirFS("views"), "*.html")
//	r.GET("/", func(c *zen.Ctx) {
//	    c.Render(200, t, "index.html", map[string]any{"title": "Home"})
//	})
func LoadTemplates(fsys fs.FS, pattern string) (*Templates, error) {
	tmpl, err := template.ParseFS(fsys, pattern)
	if err != nil {
		return nil, err
	}
	return &Templates{tmpl: tmpl}, nil
}

// Render executes a named template and writes the result to the response.
// Sets Content-Type to "text/html; charset=utf-8".
//
// Write errors are logged but not returned (they indicate connection issues).
//
// Example:
//
//	c.Render(http.StatusOK, t, "index.html", map[string]any{"title": "Home"})
func (c *Ctx) Render(status int, tmpl *Templates, name string, data any) {
	c.setContentType("text/html; charset=utf-8")
	c.Response.WriteHeader(status)

	if err := tmpl.tmpl.ExecuteTemplate(c.Response, filepath.Base(name), data); err != nil {
		logger.Error("HTTP: template render error: %v", err)
	}
}

// RenderWriter executes a named template and writes to an io.Writer.
// Useful for testing or composing templates.
//
// Example:
//
//	var buf strings.Builder
//	zen.RenderWriter(&buf, t, "index.html", data)
func RenderWriter(w io.Writer, tmpl *Templates, name string, data any) error {
	return tmpl.tmpl.ExecuteTemplate(w, filepath.Base(name), data)
}
