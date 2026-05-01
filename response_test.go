package zen

import (
	"bytes"
	"fmt"
	"mime/multipart"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestFormFile_SingleFile(t *testing.T) {
	r := New(":0")
	var capturedHeader *multipart.FileHeader
	var capturedContent []byte
	r.Handle("POST /upload", func(c *Context) {
		h, content, err := c.FormFile("file")
		if err != nil {
			c.Error(400, err.Error())
			return
		}
		capturedHeader = h
		capturedContent = content
		c.String(200, "ok")
	})

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	part, _ := writer.CreateFormFile("file", "test.txt")
	part.Write([]byte("hello world"))
	writer.Close()

	req := httptest.NewRequest("POST", "/upload", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("status = %d, want 200; body: %s", w.Code, w.Body.String())
	}
	if capturedHeader.Filename != "test.txt" {
		t.Fatalf("filename = %q, want %q", capturedHeader.Filename, "test.txt")
	}
	if string(capturedContent) != "hello world" {
		t.Fatalf("content = %q, want %q", capturedContent, "hello world")
	}
}

func TestFormFile_MissingField(t *testing.T) {
	r := New(":0")
	r.Handle("POST /upload", func(c *Context) {
		_, _, err := c.FormFile("nonexistent")
		if err != nil {
			c.Error(400, err.Error())
			return
		}
		c.String(200, "ok")
	})

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	writer.Close()

	req := httptest.NewRequest("POST", "/upload", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != 400 {
		t.Fatalf("status = %d, want 400", w.Code)
	}
}

func TestFormFiles_MultipleFiles(t *testing.T) {
	r := New(":0")
	var captured []UploadedFile
	r.Handle("POST /uploads", func(c *Context) {
		files, err := c.FormFiles("files")
		if err != nil {
			c.Error(400, err.Error())
			return
		}
		captured = files
		c.String(200, "ok")
	})

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	for i := 1; i <= 3; i++ {
		part, _ := writer.CreateFormFile("files", fmt.Sprintf("file%d.txt", i))
		part.Write([]byte(fmt.Sprintf("content%d", i)))
	}
	writer.Close()

	req := httptest.NewRequest("POST", "/uploads", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("status = %d, want 200; body: %s", w.Code, w.Body.String())
	}
	if len(captured) != 3 {
		t.Fatalf("files = %d, want 3", len(captured))
	}
	if string(captured[0].Content) != "content1" {
		t.Fatalf("content = %q", captured[0].Content)
	}
}

func TestFormFiles_NoFiles(t *testing.T) {
	r := New(":0")
	r.Handle("POST /uploads", func(c *Context) {
		_, err := c.FormFiles("files")
		if err != nil {
			c.Error(400, err.Error())
			return
		}
		c.String(200, "ok")
	})

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	writer.Close()

	req := httptest.NewRequest("POST", "/uploads", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != 400 {
		t.Fatalf("status = %d, want 400", w.Code)
	}
}

func TestContext_HTML(t *testing.T) {
	r := New(":0")
	r.Handle("GET /html", func(c *Context) {
		c.HTML(200, "<h1>Hello</h1>")
	})

	req := httptest.NewRequest("GET", "/html", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("status = %d, want 200", w.Code)
	}
	if w.Body.String() != "<h1>Hello</h1>" {
		t.Fatalf("body = %q", w.Body.String())
	}
	if ct := w.Header().Get("Content-Type"); ct != "text/html; charset=utf-8" {
		t.Fatalf("Content-Type = %q", ct)
	}
}

func TestContext_String(t *testing.T) {
	r := New(":0")
	r.Handle("GET /text", func(c *Context) {
		c.String(200, "hello world")
	})

	req := httptest.NewRequest("GET", "/text", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("status = %d, want 200", w.Code)
	}
	if w.Body.String() != "hello world" {
		t.Fatalf("body = %q", w.Body.String())
	}
	if ct := w.Header().Get("Content-Type"); ct != "text/plain; charset=utf-8" {
		t.Fatalf("Content-Type = %q", ct)
	}
}

func TestContext_NoContent(t *testing.T) {
	r := New(":0")
	r.Handle("DELETE /item", func(c *Context) {
		c.NoContent(204)
	})

	req := httptest.NewRequest("DELETE", "/item", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != 204 {
		t.Fatalf("status = %d, want 204", w.Code)
	}
	if w.Body.Len() != 0 {
		t.Fatalf("body should be empty, got %q", w.Body.String())
	}
}

func TestContext_Redirect(t *testing.T) {
	r := New(":0")
	r.Handle("GET /old", func(c *Context) {
		c.Redirect(301, "/new")
	})

	req := httptest.NewRequest("GET", "/old", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != 301 {
		t.Fatalf("status = %d, want 301", w.Code)
	}
	if loc := w.Header().Get("Location"); loc != "/new" {
		t.Fatalf("Location = %q, want %q", loc, "/new")
	}
}

func TestContext_Blob(t *testing.T) {
	r := New(":0")
	r.Handle("GET /blob", func(c *Context) {
		c.Blob(200, "text/csv", []byte("id,name\n1,John"))
	})

	req := httptest.NewRequest("GET", "/blob", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("status = %d, want 200", w.Code)
	}
	if ct := w.Header().Get("Content-Type"); ct != "text/csv" {
		t.Fatalf("Content-Type = %q", ct)
	}
	if w.Body.String() != "id,name\n1,John" {
		t.Fatalf("body = %q", w.Body.String())
	}
}

func TestContext_Stream(t *testing.T) {
	r := New(":0")
	r.Handle("GET /stream", func(c *Context) {
		reader := bytes.NewReader([]byte("streamed data"))
		c.Stream(200, "text/plain", reader)
	})

	req := httptest.NewRequest("GET", "/stream", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("status = %d, want 200", w.Code)
	}
	if ct := w.Header().Get("Content-Type"); ct != "text/plain" {
		t.Fatalf("Content-Type = %q", ct)
	}
	if w.Body.String() != "streamed data" {
		t.Fatalf("body = %q", w.Body.String())
	}
}

func TestContext_Error(t *testing.T) {
	r := New(":0")
	r.Handle("GET /err", func(c *Context) {
		c.Error(400, "bad request")
	})

	req := httptest.NewRequest("GET", "/err", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != 400 {
		t.Fatalf("status = %d, want 400", w.Code)
	}
	if ct := w.Header().Get("Content-Type"); !bytes.HasPrefix([]byte(ct), []byte("text/plain")) {
		t.Fatalf("Content-Type = %q, should be text/plain", ct)
	}
}

func TestContext_File(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "test.txt")
	os.WriteFile(path, []byte("file content"), 0644)

	r := New(":0")
	r.Handle("GET /file", func(c *Context) {
		c.File(path)
	})

	req := httptest.NewRequest("GET", "/file", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("status = %d, want 200", w.Code)
	}
	if w.Body.String() != "file content" {
		t.Fatalf("body = %q", w.Body.String())
	}
}

func TestContext_Attachment(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "test.txt")
	os.WriteFile(path, []byte("file content"), 0644)

	r := New(":0")
	r.Handle("GET /attach", func(c *Context) {
		c.Attachment(path, "custom-name.txt")
	})

	req := httptest.NewRequest("GET", "/attach", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("status = %d, want 200", w.Code)
	}
	if cd := w.Header().Get("Content-Disposition"); cd == "" {
		t.Fatal("Content-Disposition header not set")
	}
}

func TestContext_Attachment_DefaultName(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "original.txt")
	os.WriteFile(path, []byte("file content"), 0644)

	r := New(":0")
	r.Handle("GET /attach", func(c *Context) {
		c.Attachment(path, "")
	})

	req := httptest.NewRequest("GET", "/attach", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("status = %d, want 200", w.Code)
	}
	if cd := w.Header().Get("Content-Disposition"); !bytes.Contains([]byte(cd), []byte("original.txt")) {
		t.Fatalf("Content-Disposition = %q, should contain original.txt", cd)
	}
}

func TestContext_Inline(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "image.png")
	os.WriteFile(path, []byte("png data"), 0644)

	r := New(":0")
	r.Handle("GET /inline", func(c *Context) {
		c.Inline(path)
	})

	req := httptest.NewRequest("GET", "/inline", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("status = %d, want 200", w.Code)
	}
	if cd := w.Header().Get("Content-Disposition"); cd != "inline" {
		t.Fatalf("Content-Disposition = %q, want %q", cd, "inline")
	}
}

func TestContext_Body(t *testing.T) {
	r := New(":0")
	r.Handle("POST /body", func(c *Context) {
		data, err := c.Body()
		if err != nil {
			c.Error(500, err.Error())
			return
		}
		c.String(200, string(data))
	})

	req := httptest.NewRequest("POST", "/body", bytes.NewReader([]byte("raw body")))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("status = %d, want 200", w.Code)
	}
	if w.Body.String() != "raw body" {
		t.Fatalf("body = %q, want %q", w.Body.String(), "raw body")
	}
}

func TestQueryParam(t *testing.T) {
	r := New(":0")
	var captured string
	r.Handle("GET /search", func(c *Context) {
		captured = c.QueryParam("q")
		c.String(200, captured)
	})

	req := httptest.NewRequest("GET", "/search?q=golang&limit=10", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if captured != "golang" {
		t.Fatalf("q = %q, want %q", captured, "golang")
	}
}

func TestQueryParam_Missing(t *testing.T) {
	r := New(":0")
	var captured string
	r.Handle("GET /search", func(c *Context) {
		captured = c.QueryParam("missing")
		c.String(200, captured)
	})

	req := httptest.NewRequest("GET", "/search", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if captured != "" {
		t.Fatalf("missing = %q, want empty", captured)
	}
}

func TestQueryParam_FirstValue(t *testing.T) {
	r := New(":0")
	var captured string
	r.Handle("GET /multi", func(c *Context) {
		captured = c.QueryParam("item")
		c.String(200, captured)
	})

	req := httptest.NewRequest("GET", "/multi?item=first&item=second", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if captured != "first" {
		t.Fatalf("item = %q, want %q", captured, "first")
	}
}

func TestParam(t *testing.T) {
	r := New(":0")
	var captured string
	r.Handle("GET /users/{id}", func(c *Context) {
		captured = c.Param("id")
		c.String(200, captured)
	})

	req := httptest.NewRequest("GET", "/users/42", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if captured != "42" {
		t.Fatalf("id = %q, want %q", captured, "42")
	}
}

func TestParam_Missing(t *testing.T) {
	r := New(":0")
	var captured string
	r.Handle("GET /test", func(c *Context) {
		captured = c.Param("id")
		c.String(200, captured)
	})

	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if captured != "" {
		t.Fatalf("id = %q, want empty", captured)
	}
}
