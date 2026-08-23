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
	api.GET("/users/{id}", func(c *Ctx) {
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

func TestGroup_RootSlashPrefix(t *testing.T) {
	r := New(":0")

	// Group("/") must not panic and must inherit the parent prefix unchanged.
	same := r.Group("/")
	if same.prefix != "" {
		t.Fatalf("prefix = %q, want empty", same.prefix)
	}
	nested := r.Group("/api").Group("/")
	if nested.prefix != "/api" {
		t.Fatalf("prefix = %q, want %q", nested.prefix, "/api")
	}

	var called bool
	nested.GET("/users", func(c *Ctx) {
		called = true
		c.String(200, "ok")
	})

	req := httptest.NewRequest("GET", "/api/users", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("status = %d, want 200; body: %s", w.Code, w.Body.String())
	}
	if !called {
		t.Fatal("handler not called for route registered via Group(\"/\")")
	}
}

func TestGroup_HandleRaw(t *testing.T) {
	r := New(":0")
	api := r.Group("/api")
	api.HandleRaw("GET /raw", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("raw"))
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
	api := r.Group("/api", func(c *Ctx) {
		mwCalled = true
		c.Next()
	})

	var handlerCalled bool
	api.GET("/test", func(c *Ctx) {
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
	api := r.Group("/api", func(c *Ctx) {
		c.Error(403, "forbidden")
	})

	var handlerCalled bool
	api.GET("/test", func(c *Ctx) {
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
	api.Use(func(c *Ctx) {
		c.Set("auth", "token")
		c.Next()
	})

	var token string
	api.GET("/test", func(c *Ctx) {
		if val, ok := c.Get("auth"); ok {
			token = val.(string)
		}
		c.String(200, "ok")
	})

	req := httptest.NewRequest("GET", "/api/test", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if token != "token" {
		t.Fatalf("token = %q, want %q", token, "token")
	}
}

func TestGroup(t *testing.T) {
	r := New(":0")
	api := r.Group("/api")
	admin := api.Group("/admin")

	if admin.prefix != "/api/admin" {
		t.Fatalf("prefix = %q, want %q", admin.prefix, "/api/admin")
	}

	var captured string
	admin.GET("/users", func(c *Ctx) {
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

func TestGroup_MiddlewareInheritance(t *testing.T) {
	r := New(":0")
	var order []string
	api := r.Group("/api", func(c *Ctx) {
		order = append(order, "api-mw")
		c.Next()
	})
	users := api.Group("/users", func(c *Ctx) {
		order = append(order, "users-mw")
		c.Next()
	})

	users.GET("/list", func(c *Ctx) {
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

func BenchmarkGroup(b *testing.B) {
	r := New(":0")
	api := r.Group("/api")
	users := api.Group("/users")
	users.GET("/list", func(c *Ctx) {
		c.String(200, "ok")
	})

	b.ReportAllocs()

	for b.Loop() {
		req := httptest.NewRequest("GET", "/api/users/list", nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
	}
}
