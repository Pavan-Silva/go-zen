package zen

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestRouter_New(t *testing.T) {
	r := New(":8080")
	if r == nil {
		t.Fatal("New returned nil")
	}
	if r.Server == nil {
		t.Fatal("Server not initialized")
	}
	if r.Server.Addr != ":8080" {
		t.Fatalf("Addr = %q, want %q", r.Server.Addr, ":8080")
	}
}

func TestRouter_New_WithConfig(t *testing.T) {
	cfg := DefaultConfig()
	cfg.ReadTimeout = 15 * 1000000000
	r := New(":9090", cfg)

	if r.Server.ReadTimeout != 15*1000000000 {
		t.Fatalf("ReadTimeout = %v, want %v", r.Server.ReadTimeout, 15*1000000000)
	}
}

func TestRouter_New_CustomServer(t *testing.T) {
	srv := &http.Server{Addr: ":3000"}
	cfg := Config{Server: srv}
	r := New("", cfg)

	if r.Server != srv {
		t.Fatal("Custom server not used")
	}
}

func TestRouter_DefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	if cfg.ReadTimeout != 5*1000000000 {
		t.Fatalf("ReadTimeout = %v", cfg.ReadTimeout)
	}
	if cfg.WriteTimeout != 10*1000000000 {
		t.Fatalf("WriteTimeout = %v", cfg.WriteTimeout)
	}
	if cfg.IdleTimeout != 120*1000000000 {
		t.Fatalf("IdleTimeout = %v", cfg.IdleTimeout)
	}
	if cfg.ReadHeaderTimeout != 2*1000000000 {
		t.Fatalf("ReadHeaderTimeout = %v", cfg.ReadHeaderTimeout)
	}
	if cfg.MaxHeaderBytes != 1<<20 {
		t.Fatalf("MaxHeaderBytes = %v", cfg.MaxHeaderBytes)
	}
}

func TestRouter_ServeHTTP_NoMiddleware(t *testing.T) {
	r := New(":0")
	var called bool
	r.Handle("GET /test", func(c *Context) {
		called = true
		c.String(200, "ok")
	})

	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if !called {
		t.Fatal("handler not called")
	}
	if w.Code != 200 {
		t.Fatalf("status = %d, want 200", w.Code)
	}
	if w.Body.String() != "ok" {
		t.Fatalf("body = %q, want %q", w.Body.String(), "ok")
	}
}

func TestRouter_ServeHTTP_WithMiddleware(t *testing.T) {
	r := New(":0")
	var mwCalled, handlerCalled bool
	r.Use(func(c *Context, next http.Handler) {
		mwCalled = true
		next.ServeHTTP(c.Response, c.Request)
	})
	r.Handle("GET /test", func(c *Context) {
		handlerCalled = true
		c.String(200, "ok")
	})

	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if !mwCalled {
		t.Fatal("middleware not called")
	}
	if !handlerCalled {
		t.Fatal("handler not called")
	}
}

func TestRouter_Middleware_ShortCircuit(t *testing.T) {
	r := New(":0")
	var handlerCalled bool
	r.Use(func(c *Context, next http.Handler) {
		c.Error(401, "unauthorized")
	})
	r.Handle("GET /test", func(c *Context) {
		handlerCalled = true
		c.String(200, "ok")
	})

	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != 401 {
		t.Fatalf("status = %d, want 401", w.Code)
	}
	if handlerCalled {
		t.Fatal("handler should not be called after short-circuit")
	}
}

func TestRouter_Middleware_Order(t *testing.T) {
	r := New(":0")
	var order []string
	r.Use(func(c *Context, next http.Handler) {
		order = append(order, "1-before")
		next.ServeHTTP(c.Response, c.Request)
		order = append(order, "1-after")
	})
	r.Use(func(c *Context, next http.Handler) {
		order = append(order, "2-before")
		next.ServeHTTP(c.Response, c.Request)
		order = append(order, "2-after")
	})
	r.Handle("GET /test", func(c *Context) {
		order = append(order, "handler")
		c.String(200, "ok")
	})

	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	want := []string{"1-before", "2-before", "handler", "2-after", "1-after"}
	if len(order) != len(want) {
		t.Fatalf("order length = %d, want %d", len(order), len(want))
	}
	for i := range want {
		if order[i] != want[i] {
			t.Fatalf("order[%d] = %q, want %q", i, order[i], want[i])
		}
	}
}

func TestRouter_HandleRaw(t *testing.T) {
	r := New(":0")
	r.HandleRaw("GET /raw", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
		_, _ = w.Write([]byte("raw"))
	}))

	req := httptest.NewRequest("GET", "/raw", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("status = %d, want 200", w.Code)
	}
	if w.Body.String() != "raw" {
		t.Fatalf("body = %q, want %q", w.Body.String(), "raw")
	}
}

func TestRouter_File(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "test.txt")
	_ = os.WriteFile(path, []byte("file content"), 0644)

	r := New(":0")
	r.File("GET /file", path)

	req := httptest.NewRequest("GET", "/file", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("status = %d, want 200", w.Code)
	}
	if w.Body.String() != "file content" {
		t.Fatalf("body = %q, want %q", w.Body.String(), "file content")
	}
}

func TestRouter_Static(t *testing.T) {
	tmp := t.TempDir()
	_ = os.WriteFile(filepath.Join(tmp, "hello.txt"), []byte("hello"), 0644)

	r := New(":0")
	r.Static("/static", tmp)

	req := httptest.NewRequest("GET", "/static", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != 200 && w.Code != 301 {
		t.Fatalf("status = %d, want 200 or 301", w.Code)
	}
}

func TestRouter_PathParams(t *testing.T) {
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
		t.Fatalf("param = %q, want %q", captured, "42")
	}
}

func TestRouter_QueryParams(t *testing.T) {
	r := New(":0")
	var captured string
	r.Handle("GET /search", func(c *Context) {
		captured = c.QueryParam("q")
		c.String(200, captured)
	})

	req := httptest.NewRequest("GET", "/search?q=golang&page=2", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if captured != "golang" {
		t.Fatalf("query = %q, want %q", captured, "golang")
	}
}

func TestRouter_NotFound(t *testing.T) {
	r := New(":0")
	r.Handle("GET /exists", func(c *Context) {
		c.String(200, "ok")
	})

	req := httptest.NewRequest("GET", "/notfound", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != 404 {
		t.Fatalf("status = %d, want 404", w.Code)
	}
}

func TestRouter_MethodNotAllowed(t *testing.T) {
	r := New(":0")
	r.Handle("GET /only-get", func(c *Context) {
		c.String(200, "ok")
	})

	req := httptest.NewRequest("POST", "/only-get", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != 405 {
		t.Fatalf("status = %d, want 405", w.Code)
	}
}

func TestRouter_ShutdownTimeout(t *testing.T) {
	cfg := DefaultConfig()
	cfg.ShutdownTimeout = 10 * 1000000000
	r := New(":0", cfg)

	if r.shutdownTimeout != 10*1000000000 {
		t.Fatalf("shutdownTimeout = %v, want %v", r.shutdownTimeout, 10*1000000000)
	}
}

func BenchmarkRouter_ServeHTTP(b *testing.B) {
	r := New(":0")
	r.Handle("GET /bench", func(c *Context) {
		c.String(200, "ok")
	})

	req := httptest.NewRequest("GET", "/bench", nil)
	w := httptest.NewRecorder()

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		r.ServeHTTP(w, req)
	}
}

func BenchmarkRouter_ServeHTTP_WithMiddleware(b *testing.B) {
	r := New(":0")
	r.Use(func(c *Context, next http.Handler) {
		next.ServeHTTP(c.Response, c.Request)
	})
	r.Handle("GET /bench", func(c *Context) {
		c.String(200, "ok")
	})

	req := httptest.NewRequest("GET", "/bench", nil)
	w := httptest.NewRecorder()

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		r.ServeHTTP(w, req)
	}
}

func BenchmarkRouter_PathParams(b *testing.B) {
	r := New(":0")
	r.Handle("GET /users/{id}", func(c *Context) {
		c.Param("id")
		c.String(200, "ok")
	})

	req := httptest.NewRequest("GET", "/users/123", nil)
	w := httptest.NewRecorder()

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		r.ServeHTTP(w, req)
	}
}

// Test per-route middleware
func TestRouter_HandleWith_PerRouteMiddleware(t *testing.T) {
	r := New(":0")
	var middlewareCalled bool

	r.HandleWith("GET /protected", func(c *Context) {
		c.String(200, "protected")
	}, func(c *Context, next http.Handler) {
		middlewareCalled = true
		next.ServeHTTP(c.Response, c.Request)
	})

	req := httptest.NewRequest("GET", "/protected", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if !middlewareCalled {
		t.Fatal("per-route middleware not called")
	}
	if w.Code != 200 {
		t.Fatalf("status = %d, want 200", w.Code)
	}
}

func TestRouter_HandleWith_MiddlewareOrder(t *testing.T) {
	r := New(":0")
	var order []string

	r.Use(func(c *Context, next http.Handler) {
		order = append(order, "global")
		next.ServeHTTP(c.Response, c.Request)
	})

	r.HandleWith("GET /test", func(c *Context) {
		order = append(order, "handler")
		c.String(200, "ok")
	}, func(c *Context, next http.Handler) {
		order = append(order, "per-route")
		next.ServeHTTP(c.Response, c.Request)
	})

	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	want := []string{"global", "per-route", "handler"}
	if len(order) != len(want) {
		t.Fatalf("order = %v, want %v", order, want)
	}
	for i := range want {
		if order[i] != want[i] {
			t.Errorf("order[%d] = %q, want %q", i, order[i], want[i])
		}
	}
}

// Test RequestTimeout config
func TestRouter_RequestTimeout(t *testing.T) {
	cfg := DefaultConfig()
	cfg.RequestTimeout = 50 * time.Millisecond
	r := New(":0", cfg)

	if r.requestTimeout != 50*time.Millisecond {
		t.Fatalf("requestTimeout = %v, want %v", r.requestTimeout, 50*time.Millisecond)
	}
}

func TestRouter_RequestTimeout_Default(t *testing.T) {
	r := New(":0")

	if r.requestTimeout != 0 {
		t.Fatalf("requestTimeout = %v, want 0", r.requestTimeout)
	}
}
