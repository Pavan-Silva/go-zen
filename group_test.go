package zen

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestGroup_Handle(t *testing.T) {
	r := New(":0")
	api := r.Group("/api")

	var captured string
	api.Handle("GET /users/{id}", func(c *Context) {
		captured = c.Param("id")
		c.String(200, captured)
	})

	req := httptest.NewRequest("GET", "/api/users/42", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("status = %d, want 200", w.Code)
	}
	if captured != "42" {
		t.Fatalf("id = %q, want %q", captured, "42")
	}
}

func TestGroup_PrefixWithoutSlash(t *testing.T) {
	r := New(":0")
	g := r.Group("api")
	if g.prefix != "/api" {
		t.Fatalf("prefix = %q, want %q", g.prefix, "/api")
	}
}

func TestGroup_Prefix(t *testing.T) {
	r := New(":0")
	g := r.Group("/api/v1")
	if g.Prefix() != "/api/v1" {
		t.Fatalf("prefix = %q, want %q", g.Prefix(), "/api/v1")
	}
}

func TestGroup_HandleRaw(t *testing.T) {
	r := New(":0")
	api := r.Group("/api")
	api.HandleRaw("GET /raw", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("raw"))
	}))

	req := httptest.NewRequest("GET", "/api/raw", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("status = %d, want 200", w.Code)
	}
	if w.Body.String() != "raw" {
		t.Fatalf("body = %q", w.Body.String())
	}
}

func TestGroup_Middleware(t *testing.T) {
	r := New(":0")
	var mwCalled bool
	api := r.Group("/api", func(c *Context, next http.Handler) {
		mwCalled = true
		next.ServeHTTP(c.Response, c.Request)
	})

	var handlerCalled bool
	api.Handle("GET /test", func(c *Context) {
		handlerCalled = true
		c.String(200, "ok")
	})

	req := httptest.NewRequest("GET", "/api/test", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if !mwCalled {
		t.Fatal("group middleware not called")
	}
	if !handlerCalled {
		t.Fatal("handler not called")
	}
}

func TestGroup_Middleware_ShortCircuit(t *testing.T) {
	r := New(":0")
	api := r.Group("/api", func(c *Context, next http.Handler) {
		c.Error(403, "forbidden")
	})

	var handlerCalled bool
	api.Handle("GET /test", func(c *Context) {
		handlerCalled = true
		c.String(200, "ok")
	})

	req := httptest.NewRequest("GET", "/api/test", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != 403 {
		t.Fatalf("status = %d, want 403", w.Code)
	}
	if handlerCalled {
		t.Fatal("handler should not be called")
	}
}

func TestGroup_Use(t *testing.T) {
	r := New(":0")
	api := r.Group("/api")
	api.Use(func(c *Context, next http.Handler) {
		c.Set("auth", "token")
		next.ServeHTTP(c.Response, c.Request)
	})

	var token string
	api.Handle("GET /test", func(c *Context) {
		token = c.Get("auth").(string)
		c.String(200, "ok")
	})

	req := httptest.NewRequest("GET", "/api/test", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if token != "token" {
		t.Fatalf("token = %q, want %q", token, "token")
	}
}

func TestSubGroup(t *testing.T) {
	r := New(":0")
	api := r.Group("/api")
	admin := api.SubGroup("/admin")

	if admin.prefix != "/api/admin" {
		t.Fatalf("prefix = %q, want %q", admin.prefix, "/api/admin")
	}

	var captured string
	admin.Handle("GET /users", func(c *Context) {
		captured = "admin"
		c.String(200, "ok")
	})

	req := httptest.NewRequest("GET", "/api/admin/users", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("status = %d, want 200", w.Code)
	}
	if captured != "admin" {
		t.Fatal("handler not called")
	}
}

func TestSubGroup_MiddlewareInheritance(t *testing.T) {
	r := New(":0")
	var order []string
	api := r.Group("/api", func(c *Context, next http.Handler) {
		order = append(order, "api-mw")
		next.ServeHTTP(c.Response, c.Request)
	})
	users := api.SubGroup("/users", func(c *Context, next http.Handler) {
		order = append(order, "users-mw")
		next.ServeHTTP(c.Response, c.Request)
	})

	users.Handle("GET /list", func(c *Context) {
		order = append(order, "handler")
		c.String(200, "ok")
	})

	req := httptest.NewRequest("GET", "/api/users/list", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	want := []string{"api-mw", "users-mw", "handler"}
	if len(order) != len(want) {
		t.Fatalf("order = %v, want %v", order, want)
	}
	for i := range want {
		if order[i] != want[i] {
			t.Fatalf("order[%d] = %q, want %q", i, order[i], want[i])
		}
	}
}

func TestSubGroup_PrefixWithoutSlash(t *testing.T) {
	r := New(":0")
	api := r.Group("/api")
	users := api.SubGroup("users")
	if users.prefix != "/api/users" {
		t.Fatalf("prefix = %q, want %q", users.prefix, "/api/users")
	}
}

func TestCleanPath(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"/", "/"},
		{"/api//users", "/api/users"},
		{"/api/users/", "/api/users/"},
		{"///", "//"},
	}

	for _, tt := range tests {
		got := cleanPath(tt.input)
		if got != tt.want {
			t.Errorf("cleanPath(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestSplitMethodPath(t *testing.T) {
	tests := []struct {
		input    string
		wantMethod string
		wantPath  string
	}{
		{"GET /users", "GET", "/users"},
		{"POST /api/v1/users", "POST", "/api/v1/users"},
		{"/users", "", "/users"},
		{"DELETE /items/{id}", "DELETE", "/items/{id}"},
	}

	for _, tt := range tests {
		method, path := splitMethodPath(tt.input)
		if method != tt.wantMethod {
			t.Errorf("splitMethodPath(%q) method = %q, want %q", tt.input, method, tt.wantMethod)
		}
		if path != tt.wantPath {
			t.Errorf("splitMethodPath(%q) path = %q, want %q", tt.input, path, tt.wantPath)
		}
	}
}

func BenchmarkGroup(b *testing.B) {
	r := New(":0")
	api := r.Group("/api")
	users := api.SubGroup("/users")
	users.Handle("GET /list", func(c *Context) {
		c.String(200, "ok")
	})

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		req := httptest.NewRequest("GET", "/api/users/list", nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
	}
}
