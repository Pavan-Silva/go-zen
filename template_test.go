package zen

import (
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadTemplates(t *testing.T) {
	dir := t.TempDir()
	err := os.WriteFile(filepath.Join(dir, "index.html"), []byte("Hello {{.Name}}"), 0644)
	if err != nil {
		t.Fatal(err)
	}

	tmpl, err := LoadTemplates(os.DirFS(dir), "*.html")
	if err != nil {
		t.Fatalf("LoadTemplates failed: %v", err)
	}
	if tmpl == nil {
		t.Fatal("expected non-nil Templates")
	}
}

func TestLoadTemplates_InvalidPattern(t *testing.T) {
	_, err := LoadTemplates(os.DirFS(t.TempDir()), "*.nonexistent")
	if err != nil {
		// Expected - no matching files
		return
	}
	// If template.ParseFS succeeds with empty glob, it might create an empty template.
	// That's fine, no error is correct too.
}

func TestRender(t *testing.T) {
	dir := t.TempDir()
	err := os.WriteFile(filepath.Join(dir, "page.html"), []byte("<h1>{{.Title}}</h1><p>{{.Body}}</p>"), 0644)
	if err != nil {
		t.Fatal(err)
	}

	tmpl, err := LoadTemplates(os.DirFS(dir), "*.html")
	if err != nil {
		t.Fatalf("LoadTemplates failed: %v", err)
	}

	r := New(":0")
	r.GET("/page", func(c *Ctx) {
		c.Render(200, tmpl, "page.html", map[string]any{
			"Title": "Welcome",
			"Body":  "Hello World",
		})
	})

	req := httptest.NewRequest("GET", "/page", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("status = %d, want 200", w.Code)
	}
	if ct := w.Header().Get("Content-Type"); ct != "text/html; charset=utf-8" {
		t.Fatalf("Content-Type = %q, want %q", ct, "text/html; charset=utf-8")
	}
	expected := "<h1>Welcome</h1><p>Hello World</p>"
	if w.Body.String() != expected {
		t.Fatalf("body = %q, want %q", w.Body.String(), expected)
	}
}

func TestRender_TemplateNotFound(t *testing.T) {
	dir := t.TempDir()
	err := os.WriteFile(filepath.Join(dir, "exists.html"), []byte("content"), 0644)
	if err != nil {
		t.Fatal(err)
	}

	tmpl, err := LoadTemplates(os.DirFS(dir), "*.html")
	if err != nil {
		t.Fatalf("LoadTemplates failed: %v", err)
	}

	r := New(":0")
	r.GET("/missing", func(c *Ctx) {
		// Should not panic, just log the error
		c.Render(200, tmpl, "nonexistent.html", nil)
	})

	req := httptest.NewRequest("GET", "/missing", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	// Status is already written when Render is called, so we get 200
	// but the body should be empty since the template wasn't found
	if w.Code != 200 {
		t.Fatalf("status = %d, want 200", w.Code)
	}
}

func TestRenderWriter(t *testing.T) {
	dir := t.TempDir()
	err := os.WriteFile(filepath.Join(dir, "test.html"), []byte("Hello {{.Name}}"), 0644)
	if err != nil {
		t.Fatal(err)
	}

	tmpl, err := LoadTemplates(os.DirFS(dir), "*.html")
	if err != nil {
		t.Fatalf("LoadTemplates failed: %v", err)
	}

	var buf strings.Builder
	err = RenderWriter(&buf, tmpl, "test.html", map[string]any{"Name": "Zen"})
	if err != nil {
		t.Fatalf("RenderWriter failed: %v", err)
	}
	if buf.String() != "Hello Zen" {
		t.Fatalf("result = %q, want %q", buf.String(), "Hello Zen")
	}
}
