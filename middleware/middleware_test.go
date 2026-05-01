package middleware

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Pavan-Silva/go-zen"
)

func TestCORS_Default(t *testing.T) {
	r := zen.New(":0")
	r.Use(CORS(DefaultCORSConfig()))
	r.Handle("GET /api", func(c *zen.Context) {
		c.String(200, "ok")
	})

	req := httptest.NewRequest("GET", "/api", nil)
	req.Header.Set("Origin", "https://example.com")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("status = %d, want 200", w.Code)
	}
	if w.Header().Get("Access-Control-Allow-Origin") != "*" {
		t.Fatalf("ACAO = %q, want *", w.Header().Get("Access-Control-Allow-Origin"))
	}
}

func TestCORS_SpecificOrigin(t *testing.T) {
	cfg := DefaultCORSConfig()
	cfg.AllowedOrigins = []string{"https://app.example.com"}
	cfg.AllowCredentials = true

	r := zen.New(":0")
	r.Use(CORS(cfg))
	r.Handle("GET /api", func(c *zen.Context) {
		c.String(200, "ok")
	})

	req := httptest.NewRequest("GET", "/api", nil)
	req.Header.Set("Origin", "https://app.example.com")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Header().Get("Access-Control-Allow-Origin") != "https://app.example.com" {
		t.Fatalf("ACAO = %q", w.Header().Get("Access-Control-Allow-Origin"))
	}
}

func TestCORS_OriginNotAllowed(t *testing.T) {
	cfg := DefaultCORSConfig()
	cfg.AllowedOrigins = []string{"https://allowed.com"}

	r := zen.New(":0")
	r.Use(CORS(cfg))
	r.Handle("GET /api", func(c *zen.Context) {
		c.String(200, "ok")
	})

	req := httptest.NewRequest("GET", "/api", nil)
	req.Header.Set("Origin", "https://not-allowed.com")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Header().Get("Access-Control-Allow-Origin") != "" {
		t.Fatal("ACAO should not be set for disallowed origin")
	}
}

func TestCORS_Preflight(t *testing.T) {
	r := zen.New(":0")
	r.Use(CORS(DefaultCORSConfig()))
	r.Handle("GET /api", func(c *zen.Context) {
		c.String(200, "ok")
	})

	req := httptest.NewRequest("OPTIONS", "/api", nil)
	req.Header.Set("Origin", "https://example.com")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != 204 {
		t.Fatalf("status = %d, want 204", w.Code)
	}
}

func TestCORS_AllowCredentials(t *testing.T) {
	cfg := DefaultCORSConfig()
	cfg.AllowedOrigins = []string{"https://app.example.com"}
	cfg.AllowCredentials = true

	r := zen.New(":0")
	r.Use(CORS(cfg))
	r.Handle("GET /api", func(c *zen.Context) {
		c.String(200, "ok")
	})

	req := httptest.NewRequest("GET", "/api", nil)
	req.Header.Set("Origin", "https://app.example.com")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Header().Get("Access-Control-Allow-Origin") != "https://app.example.com" {
		t.Fatalf("ACAO = %q, want origin", w.Header().Get("Access-Control-Allow-Origin"))
	}
	if w.Header().Get("Access-Control-Allow-Credentials") != "true" {
		t.Fatal("ACAC should be true")
	}
}

func TestCORS_NoCredentials_UsesWildcard(t *testing.T) {
	cfg := DefaultCORSConfig()
	cfg.AllowCredentials = false

	r := zen.New(":0")
	r.Use(CORS(cfg))
	r.Handle("GET /api", func(c *zen.Context) {
		c.String(200, "ok")
	})

	req := httptest.NewRequest("GET", "/api", nil)
	req.Header.Set("Origin", "https://example.com")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Header().Get("Access-Control-Allow-Origin") != "*" {
		t.Fatalf("ACAO = %q, want *", w.Header().Get("Access-Control-Allow-Origin"))
	}
}

func TestBodyLimit_String(t *testing.T) {
	r := zen.New(":0")
	r.Use(BodyLimit("1K"))
	r.Handle("POST /upload", func(c *zen.Context) {
		c.String(200, "ok")
	})

	body := strings.NewReader(strings.Repeat("x", 2000))
	req := httptest.NewRequest("POST", "/upload", body)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != 413 {
		t.Fatalf("status = %d, want 413", w.Code)
	}
}

func TestBodyLimit_Int64(t *testing.T) {
	r := zen.New(":0")
	r.Use(BodyLimit(int64(100)))
	r.Handle("POST /upload", func(c *zen.Context) {
		c.String(200, "ok")
	})

	body := strings.NewReader(strings.Repeat("x", 200))
	req := httptest.NewRequest("POST", "/upload", body)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != 413 {
		t.Fatalf("status = %d, want 413", w.Code)
	}
}

func TestBodyLimit_Int(t *testing.T) {
	r := zen.New(":0")
	r.Use(BodyLimit(100))
	r.Handle("POST /upload", func(c *zen.Context) {
		c.String(200, "ok")
	})

	body := strings.NewReader(strings.Repeat("x", 50))
	req := httptest.NewRequest("POST", "/upload", body)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("status = %d, want 200", w.Code)
	}
}

func TestBodyLimit_Skipper(t *testing.T) {
	cfg := BodyLimitConfig()
	cfg.Limit = "1K"
	cfg.Skipper = func(r *http.Request) bool {
		return r.URL.Path == "/upload/large"
	}

	r := zen.New(":0")
	r.Use(BodyLimitWithConfig(cfg))
	r.Handle("POST /upload/large", func(c *zen.Context) {
		c.String(200, "ok")
	})

	body := strings.NewReader(strings.Repeat("x", 2000))
	req := httptest.NewRequest("POST", "/upload/large", body)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("status = %d, want 200", w.Code)
	}
}

func TestBodyLimit_ContentLength(t *testing.T) {
	r := zen.New(":0")
	r.Use(BodyLimit("1K"))
	r.Handle("POST /upload", func(c *zen.Context) {
		c.String(200, "ok")
	})

	req := httptest.NewRequest("POST", "/upload", nil)
	req.ContentLength = 2000
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != 413 {
		t.Fatalf("status = %d, want 413", w.Code)
	}
}

func TestRecover(t *testing.T) {
	r := zen.New(":0")
	r.Use(Recover)
	r.Handle("GET /panic", func(c *zen.Context) {
		panic("test panic")
	})

	req := httptest.NewRequest("GET", "/panic", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != 500 {
		t.Fatalf("status = %d, want 500", w.Code)
	}
}

func TestLogger(t *testing.T) {
	r := zen.New(":0")
	r.Use(Logger)
	r.Handle("GET /log", func(c *zen.Context) {
		c.String(200, "ok")
	})

	req := httptest.NewRequest("GET", "/log", nil)
	req.RemoteAddr = "127.0.0.1:12345"
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("status = %d, want 200", w.Code)
	}
}

func TestClientIP_XForwardedFor(t *testing.T) {
	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("X-Forwarded-For", "1.2.3.4, 5.6.7.8")
	req.RemoteAddr = "127.0.0.1:12345"

	got := clientIP(req)
	if got != "1.2.3.4" {
		t.Fatalf("ip = %q, want %q", got, "1.2.3.4")
	}
}

func TestClientIP_XRealIP(t *testing.T) {
	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("X-Real-IP", "10.0.0.1")
	req.RemoteAddr = "127.0.0.1:12345"

	got := clientIP(req)
	if got != "10.0.0.1" {
		t.Fatalf("ip = %q, want %q", got, "10.0.0.1")
	}
}

func TestClientIP_RemoteAddr(t *testing.T) {
	req := httptest.NewRequest("GET", "/", nil)
	req.RemoteAddr = "127.0.0.1:12345"

	got := clientIP(req)
	if got != "127.0.0.1:12345" {
		t.Fatalf("ip = %q, want %q", got, "127.0.0.1:12345")
	}
}

func TestParseLimit(t *testing.T) {
	tests := []struct {
		input string
		want  int64
	}{
		{"1K", 1024},
		{"2M", 2 * 1024 * 1024},
		{"1G", 1024 * 1024 * 1024},
		{"512", 512},
		{"10k", 10 * 1024},
	}

	for _, tt := range tests {
		got, err := parseLimit(tt.input)
		if err != nil {
			t.Fatalf("parseLimit(%q) error: %v", tt.input, err)
		}
		if got != tt.want {
			t.Errorf("parseLimit(%q) = %d, want %d", tt.input, got, tt.want)
		}
	}
}
