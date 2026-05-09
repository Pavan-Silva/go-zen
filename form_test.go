package zen

import (
	"fmt"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

type testFormUser struct {
	Name  string `form:"name" json:"name" validate:"required"`
	Email string `form:"email" json:"email" validate:"email"`
	Age   int    `form:"age" json:"age" validate:"gte=0"`
	Flag  bool   `form:"flag" json:"flag"`
}

type testFormJSONFallback struct {
	Name string `json:"name"`
	Age  int    `json:"age"`
}

func TestBindForm_Valid(t *testing.T) {
	r := New(":0")
	var captured testFormUser
	r.Handle("POST /form", func(c *Context) {
		if err := c.BindForm(&captured); err != nil {
			c.Error(400, err.Error())
			return
		}
		c.JSON(200, captured)
	})

	data := url.Values{}
	data.Set("name", "John")
	data.Set("email", "john@example.com")
	data.Set("age", "30")
	data.Set("flag", "true")

	req := httptest.NewRequest("POST", "/form", strings.NewReader(data.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("status = %d, want 200; body: %s", w.Code, w.Body.String())
	}
	if captured.Name != "John" {
		t.Fatalf("name = %q, want %q", captured.Name, "John")
	}
	if captured.Age != 30 {
		t.Fatalf("age = %d, want 30", captured.Age)
	}
	if !captured.Flag {
		t.Fatal("flag should be true")
	}
}

func TestBindForm_JSONTagFallback(t *testing.T) {
	r := New(":0")
	var captured testFormJSONFallback
	r.Handle("POST /form", func(c *Context) {
		if err := c.BindForm(&captured); err != nil {
			c.Error(400, err.Error())
			return
		}
		c.JSON(200, captured)
	})

	data := url.Values{}
	data.Set("name", "Jane")
	data.Set("age", "25")

	req := httptest.NewRequest("POST", "/form", strings.NewReader(data.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("status = %d, want 200; body: %s", w.Code, w.Body.String())
	}
	if captured.Name != "Jane" {
		t.Fatalf("name = %q, want %q", captured.Name, "Jane")
	}
}

func TestBindForm_InvalidType(t *testing.T) {
	r := New(":0")
	var captured testFormUser
	r.Handle("POST /form", func(c *Context) {
		if err := c.BindForm(&captured); err != nil {
			c.Error(400, err.Error())
			return
		}
		c.String(200, "ok")
	})

	data := url.Values{}
	data.Set("age", "not-a-number")

	req := httptest.NewRequest("POST", "/form", strings.NewReader(data.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != 400 {
		t.Fatalf("status = %d, want 400", w.Code)
	}
}

func TestBindForm_Validation(t *testing.T) {
	r := New(":0")
	var captured testFormUser
	r.Handle("POST /form", func(c *Context) {
		if err := c.BindForm(&captured); err != nil {
			c.Error(400, err.Error())
			return
		}
		if err := Validate(&captured); err != nil {
			c.Error(400, err.Error())
			return
		}
		c.String(200, "ok")
	})

	data := url.Values{}
	data.Set("name", "")
	data.Set("email", "invalid")

	req := httptest.NewRequest("POST", "/form", strings.NewReader(data.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != 400 {
		t.Fatalf("status = %d, want 400; body: %s", w.Code, w.Body.String())
	}
}

func TestBindForm_InvalidTarget(t *testing.T) {
	r := New(":0")
	var captured map[string]string
	r.Handle("POST /form", func(c *Context) {
		if err := c.BindForm(&captured); err != nil {
			c.Error(400, err.Error())
			return
		}
		c.String(200, "ok")
	})

	data := url.Values{}
	data.Set("key", "value")

	req := httptest.NewRequest("POST", "/form", strings.NewReader(data.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != 400 {
		t.Fatalf("status = %d, want 400", w.Code)
	}
}

func TestBindForm_BoolVariants(t *testing.T) {
	type BoolForm struct {
		A bool `form:"a"`
		B bool `form:"b"`
		C bool `form:"c"`
	}

	r := New(":0")
	var captured BoolForm
	r.Handle("POST /bools", func(c *Context) {
		if err := c.BindForm(&captured); err != nil {
			c.Error(400, err.Error())
			return
		}
		c.JSON(200, captured)
	})

	data := url.Values{}
	data.Set("a", "TRUE")
	data.Set("b", "yes")
	data.Set("c", "on")

	req := httptest.NewRequest("POST", "/bools", strings.NewReader(data.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("status = %d, want 200; body: %s", w.Code, w.Body.String())
	}
	if !captured.A || !captured.B || !captured.C {
		t.Fatalf("bools = %+v, want all true", captured)
	}
}

func TestBindForm_MissingFields(t *testing.T) {
	type PartialForm struct {
		Name  string `form:"name" json:"name"`
		Email string `form:"email" json:"email"`
	}

	r := New(":0")
	var captured PartialForm
	r.Handle("POST /form", func(c *Context) {
		if err := c.BindForm(&captured); err != nil {
			c.Error(400, err.Error())
			return
		}
		c.JSON(200, captured)
	})

	data := url.Values{}
	data.Set("name", "John")

	req := httptest.NewRequest("POST", "/form", strings.NewReader(data.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("status = %d, want 200", w.Code)
	}
	if captured.Name != "John" {
		t.Fatalf("name = %q, want %q", captured.Name, "John")
	}
	if captured.Email != "" {
		t.Fatalf("email = %q, want empty", captured.Email)
	}
}

func TestFormError_Error(t *testing.T) {
	err := &FormError{Field: "age", Err: fmt.Errorf("invalid value")}
	msg := err.Error()
	if !strings.Contains(msg, "age") {
		t.Fatalf("error message should contain field name: %s", msg)
	}
}

func TestFormError_Unwrap(t *testing.T) {
	innerErr := &FormError{Field: "age", Err: fmt.Errorf("cause")}
	if innerErr.Unwrap() == nil {
		t.Fatal("Unwrap should return the cause error")
	}
}

func BenchmarkBindForm(b *testing.B) {
	r := New(":0")
	var captured testFormUser
	r.Handle("POST /form", func(c *Context) {
		_ = c.BindForm(&captured)
		c.String(200, "ok")
	})

	data := url.Values{}
	data.Set("name", "John")
	data.Set("email", "john@example.com")
	data.Set("age", "30")
	data.Set("flag", "true")

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		req := httptest.NewRequest("POST", "/form", strings.NewReader(data.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
	}
}
