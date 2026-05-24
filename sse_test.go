package zen

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestEncodeSSEData_String(t *testing.T) {
	result, err := encodeSSEData("hello world")
	if err != nil {
		t.Fatal(err)
	}
	if result != "hello world" {
		t.Fatalf("result = %q, want %q", result, "hello world")
	}
}

func TestEncodeSSEData_StringWithNewlines(t *testing.T) {
	result, err := encodeSSEData("line1\nline2")
	if err != nil {
		t.Fatal(err)
	}
	if result != "line1\ndata: line2" {
		t.Fatalf("result = %q, want %q", result, "line1\ndata: line2")
	}
}

func TestEncodeSSEData_Bytes(t *testing.T) {
	result, err := encodeSSEData([]byte("binary data"))
	if err != nil {
		t.Fatal(err)
	}
	if result != "binary data" {
		t.Fatalf("result = %q", result)
	}
}

func TestEncodeSSEData_BytesWithNewlines(t *testing.T) {
	result, err := encodeSSEData([]byte("a\nb"))
	if err != nil {
		t.Fatal(err)
	}
	if result != "a\ndata: b" {
		t.Fatalf("result = %q", result)
	}
}

func TestEncodeSSEData_JSON(t *testing.T) {
	type msg struct {
		ID   int    `json:"id"`
		Text string `json:"text"`
	}
	result, err := encodeSSEData(msg{ID: 1, Text: "hello"})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(result, `"id":1`) && !strings.Contains(result, `"id": 1`) {
		t.Fatalf("result should contain id field: %q", result)
	}
}

func TestEncodeSSEData_CRLFNormalization(t *testing.T) {
	result, err := encodeSSEData("line1\r\nline2\rline3")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(result, "\r") {
		t.Fatalf("result should not contain \\r: %q", result)
	}
}

func TestSSEvent_FullHTTP(t *testing.T) {
	r := New(":0")
	r.Handle("GET /events", func(c *Ctx) {
		c.Response.Header().Set("Content-Type", "text/event-stream")
		if err := c.SSEvent("update", "hello"); err != nil {
			t.Errorf("SSEvent error: %v", err)
		}
	})

	req := httptest.NewRequest("GET", "/events", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("status = %d, want 200", w.Code)
	}
	body := w.Body.String()
	if !strings.Contains(body, "event: update") {
		t.Fatalf("body should contain event name: %s", body)
	}
	if !strings.Contains(body, "data: hello") {
		t.Fatalf("body should contain data: %s", body)
	}
}

func TestSSEvent_JSON(t *testing.T) {
	r := New(":0")
	r.Handle("GET /events", func(c *Ctx) {
		c.Response.Header().Set("Content-Type", "text/event-stream")
		if err := c.SSEvent("", map[string]string{"msg": "hello"}); err != nil {
			t.Errorf("SSEvent error: %v", err)
		}
	})

	req := httptest.NewRequest("GET", "/events", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("status = %d, want 200", w.Code)
	}
	body := w.Body.String()
	if !strings.Contains(body, `"msg"`) {
		t.Fatalf("body should contain JSON: %s", body)
	}
}

func TestSSEvent_Headers(t *testing.T) {
	r := New(":0")
	r.Handle("GET /events", func(c *Ctx) {
		if err := c.SSEvent("msg", "hello"); err != nil {
			t.Errorf("SSEvent error: %v", err)
		}
	})

	req := httptest.NewRequest("GET", "/events", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("status = %d, want 200", w.Code)
	}
	ct := w.Header().Get("Content-Type")
	if ct != "text/event-stream; charset=utf-8" {
		t.Fatalf("Content-Type = %q, want text/event-stream", ct)
	}
	if w.Header().Get("Cache-Control") != "no-cache" {
		t.Fatalf("Cache-Control = %q, want no-cache", w.Header().Get("Cache-Control"))
	}
	if w.Header().Get("Connection") != "keep-alive" {
		t.Fatalf("Connection = %q, want keep-alive", w.Header().Get("Connection"))
	}
}

func TestSSEvent_ErrFlusherUnsupported(t *testing.T) {
	c := &Ctx{Response: &noFlushWriter{}}
	err := c.SSEvent("msg", "data")
	if err != ErrFlusherUnsupported {
		t.Fatalf("error = %v, want %v", err, ErrFlusherUnsupported)
	}
}

type noFlushWriter struct{}

func (w *noFlushWriter) Header() http.Header         { return http.Header{} }
func (w *noFlushWriter) Write(b []byte) (int, error) { return len(b), nil }
func (w *noFlushWriter) WriteHeader(int)             {}

func TestSSEvent_WriteError(t *testing.T) {
	c := &Ctx{Response: &errWriter{}}
	err := c.SSEvent("msg", "data")
	if err == nil {
		t.Fatal("expected write error")
	}
}

type errWriter struct{}

func (w *errWriter) Header() http.Header         { return http.Header{} }
func (w *errWriter) Write([]byte) (int, error)   { return 0, io.ErrUnexpectedEOF }
func (w *errWriter) WriteHeader(int)             {}
func (w *errWriter) Flush()                      {}

func TestHasSSEContentType(t *testing.T) {
	tests := []struct {
		input string
		want  bool
	}{
		{"text/event-stream", true},
		{"text/event-stream; charset=utf-8", true},
		{"TEXT/EVENT-STREAM", true},
		{"text/event-stream ", true},
		{"application/json", false},
		{"", false},
	}

	for _, tt := range tests {
		got := hasSSEContentType(tt.input)
		if got != tt.want {
			t.Errorf("hasSSEContentType(%q) = %v, want %v", tt.input, got, tt.want)
		}
	}
}
