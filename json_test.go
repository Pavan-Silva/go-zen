package zen

import (
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"
)

type testJSONUser struct {
	Name  string `json:"name" validate:"required"`
	Email string `json:"email" validate:"required,email"`
	Age   int    `json:"age" validate:"gte=0"`
}

func TestBindJSON_Valid(t *testing.T) {
	r := New(":0")
	var captured testJSONUser
	r.POST("/user", func(c *Ctx) {
		if err := c.BindJSON(&captured); err != nil {
			c.Error(400, err.Error())
			return
		}
		c.JSON(200, captured)
	})

	body := strings.NewReader(`{"name":"John","email":"john@example.com","age":30}`)
	req := httptest.NewRequest("POST", "/user", body)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("status = %d, want 200; body: %s", w.Code, w.Body.String())
	}
	if captured.Name != "John" {
		t.Fatalf("name = %q, want %q", captured.Name, "John")
	}
}

func TestBindJSON_Malformed(t *testing.T) {
	r := New(":0")
	var captured testJSONUser
	r.PATCH("/user", func(c *Ctx) {
		if err := c.BindJSON(&captured); err != nil {
			c.Error(400, err.Error())
			return
		}
		c.String(200, "ok")
	})

	body := strings.NewReader(`{invalid json}`)
	req := httptest.NewRequest("PATCH", "/user", body)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != 400 {
		t.Fatalf("status = %d, want 400", w.Code)
	}
}

func TestBindJSON_TrailingData(t *testing.T) {
	r := New(":0")
	var captured testJSONUser
	r.POST("/user", func(c *Ctx) {
		if err := c.BindJSON(&captured); err != nil {
			c.Error(400, err.Error())
			return
		}
		c.String(200, "ok")
	})

	body := strings.NewReader(`{"name":"John","email":"john@example.com","age":30}{"name":"Jane"}`)
	req := httptest.NewRequest("POST", "/user", body)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("status = %d, want 200 (decoder ignores trailing data)", w.Code)
	}
}

func TestBindJSON_Validation(t *testing.T) {
	r := New(":0")
	var captured testJSONUser
	r.POST("/user", func(c *Ctx) {
		if err := c.BindJSON(&captured); err != nil {
			c.Error(400, err.Error())
			return
		}
		if err := Validate(&captured); err != nil {
			c.Error(400, err.Error())
			return
		}
		c.String(200, "ok")
	})

	body := strings.NewReader(`{"name":"","email":"invalid"}`)
	req := httptest.NewRequest("POST", "/user", body)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != 400 {
		t.Fatalf("status = %d, want 400; body: %s", w.Code, w.Body.String())
	}
}

func TestBindJSON_NonStruct(t *testing.T) {
	r := New(":0")
	var captured map[string]any
	r.POST("/map", func(c *Ctx) {
		if err := c.BindJSON(&captured); err != nil {
			c.Error(400, err.Error())
			return
		}
		c.JSON(200, captured)
	})

	body := strings.NewReader(`{"key":"value"}`)
	req := httptest.NewRequest("POST", "/map", body)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("status = %d, want 200", w.Code)
	}
	if captured["key"] != "value" {
		t.Fatalf("map = %v", captured)
	}
}

func TestJSON_Response(t *testing.T) {
	r := New(":0")
	r.GET("/json", func(c *Ctx) {
		c.JSON(201, map[string]any{"id": 1, "name": "test"})
	})

	req := httptest.NewRequest("GET", "/json", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != 201 {
		t.Fatalf("status = %d, want 201", w.Code)
	}
	if ct := w.Header().Get("Content-Type"); ct != "application/json" {
		t.Fatalf("Content-Type = %q, want %q", ct, "application/json")
	}

	var resp map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("response not valid JSON: %v", err)
	}
	if resp["name"] != "test" {
		t.Fatalf("name = %v", resp["name"])
	}
}

func TestJSON_EncodeError(t *testing.T) {
	r := New(":0")
	r.GET("/bad", func(c *Ctx) {
		c.JSON(200, make(chan int))
	})

	req := httptest.NewRequest("GET", "/bad", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	// Without delayedStatusWriter, the status (200) is already sent before
	// the encode error occurs. The error is logged but status can't be changed.
	if w.Code != 200 {
		t.Fatalf("status = %d, want 200 (status already sent before encode error)", w.Code)
	}
}

func BenchmarkBindJSON(b *testing.B) {
	r := New(":0")
	var captured testJSONUser
	r.POST("/user", func(c *Ctx) {
		_ = c.BindJSON(&captured)
		c.String(200, "ok")
	})

	body := strings.NewReader(`{"name":"John","email":"john@example.com","age":30}`)

	b.ReportAllocs()
	
	for b.Loop() {
		req := httptest.NewRequest("POST", "/user", body)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
	}
}

func BenchmarkJSON(b *testing.B) {
	r := New(":0")
	r.GET("/json", func(c *Ctx) {
		c.JSON(200, map[string]any{"id": 1, "name": "benchmark", "active": true})
	})

	b.ReportAllocs()
	
	for b.Loop() {
		req := httptest.NewRequest("GET", "/json", nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
	}
}
