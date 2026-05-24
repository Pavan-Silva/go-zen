package zen

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestBindBody_JSON(t *testing.T) {
	type payload struct {
		Name  string `json:"name"`
		Email string `json:"email"`
	}

	r := New(":0")
	r.Handle("POST /body", func(c *Ctx) {
		var p payload
		if err := c.BindBody(&p); err != nil {
			c.Error(http.StatusBadRequest, err.Error())
			return
		}
		c.JSON(http.StatusOK, p)
	})

	body := `{"name":"John","email":"john@example.com"}`
	req := httptest.NewRequest("POST", "/body", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("status = %d, want 200; body: %s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "John") {
		t.Fatalf("body should contain name: %s", w.Body.String())
	}
}

func TestBindBody_XML(t *testing.T) {
	type payload struct {
		Name string `xml:"name"`
	}

	r := New(":0")
	r.Handle("POST /body", func(c *Ctx) {
		var p payload
		if err := c.BindBody(&p); err != nil {
			c.Error(http.StatusBadRequest, err.Error())
			return
		}
		c.XML(http.StatusOK, p)
	})

	body := `<payload><name>John</name></payload>`
	req := httptest.NewRequest("POST", "/body", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/xml")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("status = %d, want 200; body: %s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "John") {
		t.Fatalf("body should contain name: %s", w.Body.String())
	}
}

func TestBindBody_Form(t *testing.T) {
	type payload struct {
		Name  string `form:"name"`
		Email string `form:"email"`
	}

	r := New(":0")
	r.Handle("POST /body", func(c *Ctx) {
		var p payload
		if err := c.BindBody(&p); err != nil {
			c.Error(http.StatusBadRequest, err.Error())
			return
		}
		c.JSON(http.StatusOK, p)
	})

	body := "name=John&email=john@example.com"
	req := httptest.NewRequest("POST", "/body", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("status = %d, want 200; body: %s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "John") {
		t.Fatalf("body should contain name: %s", w.Body.String())
	}
}

func TestBindBody_PlusJSON(t *testing.T) {
	type payload struct {
		Name string `json:"name"`
	}

	r := New(":0")
	r.Handle("POST /body", func(c *Ctx) {
		var p payload
		if err := c.BindBody(&p); err != nil {
			c.Error(http.StatusBadRequest, err.Error())
			return
		}
		c.JSON(http.StatusOK, p)
	})

	body := `{"name":"test"}`
	req := httptest.NewRequest("POST", "/body", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/vnd.api+json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("status = %d, want 200; body: %s", w.Code, w.Body.String())
	}
}

func TestBindBody_Empty(t *testing.T) {
	r := New(":0")
	r.Handle("GET /body", func(c *Ctx) {
		if err := c.BindBody(nil); err != nil {
			c.Error(http.StatusBadRequest, err.Error())
			return
		}
		c.String(http.StatusOK, "ok")
	})

	req := httptest.NewRequest("GET", "/body", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("status = %d, want 200", w.Code)
	}
}

func TestBindBody_Unsupported(t *testing.T) {
	r := New(":0")
	r.Handle("POST /body", func(c *Ctx) {
		var p struct{}
		if err := c.BindBody(&p); err != nil {
			c.String(http.StatusBadRequest, err.Error())
			return
		}
		c.String(http.StatusOK, "ok")
	})

	req := httptest.NewRequest("POST", "/body", strings.NewReader("data"))
	req.Header.Set("Content-Type", "text/plain")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != 400 {
		t.Fatalf("status = %d, want 400; body: %s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "unsupported") {
		t.Fatalf("should mention unsupported: %s", w.Body.String())
	}
}

func TestBind_Body(t *testing.T) {
	type payload struct {
		Name string `json:"name"`
	}

	r := New(":0")
	r.Handle("POST /bind", func(c *Ctx) {
		var p payload
		if err := c.Bind(&p); err != nil {
			c.Error(http.StatusBadRequest, err.Error())
			return
		}
		c.JSON(http.StatusOK, p)
	})

	body := `{"name":"John"}`
	req := httptest.NewRequest("POST", "/bind", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("status = %d, want 200; body: %s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "John") {
		t.Fatalf("body should contain name: %s", w.Body.String())
	}
}

func TestBind_GET(t *testing.T) {
	type query struct {
		Page  int    `query:"page"`
		Limit int    `query:"limit"`
		Name  string `query:"name"`
	}

	r := New(":0")
	r.Handle("GET /search", func(c *Ctx) {
		var q query
		if err := c.Bind(&q); err != nil {
			c.Error(http.StatusBadRequest, err.Error())
			return
		}
		c.JSON(http.StatusOK, q)
	})

	req := httptest.NewRequest("GET", "/search?page=2&limit=20&name=John", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("status = %d, want 200; body: %s", w.Code, w.Body.String())
	}
}

func TestBind_NoBody(t *testing.T) {
	r := New(":0")
	r.Handle("POST /nobody", func(c *Ctx) {
		if err := c.Bind(nil); err != nil {
			c.Error(http.StatusBadRequest, err.Error())
			return
		}
		c.String(http.StatusOK, "ok")
	})

	req := httptest.NewRequest("POST", "/nobody", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("status = %d, want 200", w.Code)
	}
}

func TestBindPathParams(t *testing.T) {
	type params struct {
		UserID string `param:"userID"`
		PostID string `param:"postID"`
	}

	r := New(":0")
	r.Handle("GET /users/{userID}/posts/{postID}", func(c *Ctx) {
		var p params
		if err := c.BindPathParams(&p); err != nil {
			c.Error(http.StatusBadRequest, err.Error())
			return
		}
		c.JSON(http.StatusOK, p)
	})

	req := httptest.NewRequest("GET", "/users/u1/posts/p2", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("status = %d, want 200; body: %s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "u1") || !strings.Contains(w.Body.String(), "p2") {
		t.Fatalf("body should contain params: %s", w.Body.String())
	}
}

func TestBindPathParams_NoMatch(t *testing.T) {
	type params struct {
		UserID string `param:"userID"`
	}

	r := New(":0")
	r.Handle("GET /users/{userID}", func(c *Ctx) {
		var p params
		if err := c.BindPathParams(&p); err != nil {
			c.Error(http.StatusBadRequest, err.Error())
			return
		}
		c.String(http.StatusOK, p.UserID)
	})

	req := httptest.NewRequest("GET", "/users/abc", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("status = %d, want 200; body: %s", w.Code, w.Body.String())
	}
	if w.Body.String() != "abc" {
		t.Fatalf("body = %q, want %q", w.Body.String(), "abc")
	}
}

func TestBindPathParams_NonStruct(t *testing.T) {
	r := New(":0")
	r.Handle("GET /users/{id}", func(c *Ctx) {
		var s string
		if err := c.BindPathParams(&s); err != nil {
			c.Error(http.StatusBadRequest, err.Error())
			return
		}
		c.String(http.StatusOK, "ok")
	})

	req := httptest.NewRequest("GET", "/users/1", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("status = %d, want 200", w.Code)
	}
}

func TestBindPathParams_Int(t *testing.T) {
	type params struct {
		ID int `param:"id"`
	}

	r := New(":0")
	r.Handle("GET /items/{id}", func(c *Ctx) {
		var p params
		if err := c.BindPathParams(&p); err != nil {
			c.Error(http.StatusBadRequest, err.Error())
			return
		}
		c.JSON(http.StatusOK, p)
	})

	req := httptest.NewRequest("GET", "/items/42", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("status = %d, want 200; body: %s", w.Code, w.Body.String())
	}
}

func TestBindQueryParams(t *testing.T) {
	type query struct {
		Page    int      `query:"page"`
		Tag     string   `query:"tag"`
		Filters []string `query:"filter"`
	}

	r := New(":0")
	r.Handle("GET /items", func(c *Ctx) {
		var q query
		if err := c.BindQueryParams(&q); err != nil {
			c.Error(http.StatusBadRequest, err.Error())
			return
		}
		c.JSON(http.StatusOK, q)
	})

	req := httptest.NewRequest("GET", "/items?page=1&tag=golang&filter=a&filter=b", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("status = %d, want 200; body: %s", w.Code, w.Body.String())
	}
}

func TestBindQueryParams_Missing(t *testing.T) {
	type query struct {
		Page int `query:"page"`
	}

	r := New(":0")
	r.Handle("GET /items", func(c *Ctx) {
		var q query
		if err := c.BindQueryParams(&q); err != nil {
			c.Error(http.StatusBadRequest, err.Error())
			return
		}
		c.JSON(http.StatusOK, q)
	})

	req := httptest.NewRequest("GET", "/items", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("status = %d, want 200", w.Code)
	}
}

func TestBindQueryParams_NonStruct(t *testing.T) {
	r := New(":0")
	r.Handle("GET /items", func(c *Ctx) {
		var s string
		if err := c.BindQueryParams(&s); err != nil {
			c.Error(http.StatusBadRequest, err.Error())
			return
		}
		c.String(http.StatusOK, "ok")
	})

	req := httptest.NewRequest("GET", "/items", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("status = %d, want 200", w.Code)
	}
}

func TestBind_WithPathAndQuery(t *testing.T) {
	type bindReq struct {
		ID   string `param:"id"`
		Page int    `query:"page"`
		Name string `json:"name"`
	}

	r := New(":0")
	r.Handle("POST /items/{id}", func(c *Ctx) {
		var p bindReq
		if err := c.Bind(&p); err != nil {
			c.Error(http.StatusBadRequest, err.Error())
			return
		}
		c.JSON(http.StatusOK, p)
	})

	body := `{"name":"test"}`
	req := httptest.NewRequest("POST", "/items/42?page=3", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("status = %d, want 200; body: %s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "42") || !strings.Contains(w.Body.String(), "test") {
		t.Fatalf("body should contain id and name: %s", w.Body.String())
	}
}
