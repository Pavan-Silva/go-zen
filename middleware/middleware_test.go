package middleware

import (
	"compress/gzip"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/Pavan-Silva/go-zen"
)

func TestDefaultBodyLimitConfig(t *testing.T) {
	cfg := DefaultBodyLimitConfig()
	if cfg.Limit != "2M" {
		t.Fatalf("Limit = %v, want %q", cfg.Limit, "2M")
	}
	if cfg.Skipper != nil {
		t.Fatal("Skipper should be nil by default")
	}
}

func TestCORS_Default(t *testing.T) {
	r := zen.New(":0")
	r.Use(CORS(DefaultCORSConfig()))
	r.GET("/api", func(c *zen.Ctx) {
		c.String(200, "ok")
	})

	req := httptest.NewRequest("GET", "/api", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("status = %d, want 200", w.Code)
	}
	// No Origin header → pass through, no CORS headers
	if w.Header().Get("Access-Control-Allow-Origin") != "" {
		t.Fatalf("ACAO should not be set")
	}
}

func TestCORS_SpecificOrigin(t *testing.T) {
	cfg := DefaultCORSConfig()
	cfg.AllowedOrigins = []string{"https://app.example.com"}
	cfg.AllowCredentials = true

	r := zen.New(":0")
	r.Use(CORS(cfg))
	r.GET("/api", func(c *zen.Ctx) {
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
	r.GET("/api", func(c *zen.Ctx) {
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
	cfg := DefaultCORSConfig()
	cfg.AllowedOrigins = []string{"https://example.com"}

	r := zen.New(":0")
	r.Use(CORS(cfg))
	r.GET("/api", func(c *zen.Ctx) {
		c.String(200, "ok")
	})
	r.OPTIONS("/api", func(c *zen.Ctx) {
		c.Status(http.StatusNoContent)
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
	r.GET("/api", func(c *zen.Ctx) {
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
	cfg.AllowedOrigins = []string{"*"}
	cfg.AllowCredentials = false

	r := zen.New(":0")
	r.Use(CORS(cfg))
	r.GET("/api", func(c *zen.Ctx) {
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

// CORS must append to an existing Vary header, not overwrite it.
func TestCORS_VaryPreserved(t *testing.T) {
	cfg := DefaultCORSConfig()
	cfg.AllowedOrigins = []string{"https://example.com"}

	r := zen.New(":0")
	r.Use(CORS(cfg))
	r.GET("/api", func(c *zen.Ctx) {
		c.Response.Header().Add("Vary", "X-Custom")
		c.String(200, "ok")
	})

	req := httptest.NewRequest("GET", "/api", nil)
	req.Header.Set("Origin", "https://example.com")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	vals := w.Header().Values("Vary")
	if !slices.Contains(vals, "Origin") || !slices.Contains(vals, "X-Custom") {
		t.Fatalf("Vary = %v, want both Origin and X-Custom", vals)
	}
}

func TestBodyLimit_String(t *testing.T) {
	r := zen.New(":0")
	r.Use(BodyLimit("1K"))
	r.POST("/upload", func(c *zen.Ctx) {
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
	r.POST("/upload", func(c *zen.Ctx) {
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
	r.POST("/upload", func(c *zen.Ctx) {
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
	cfg := DefaultBodyLimitConfig()
	cfg.Limit = "1K"
	cfg.Skipper = func(r *http.Request) bool {
		return r.URL.Path == "/upload/large"
	}

	r := zen.New(":0")
	r.Use(BodyLimitWithConfig(cfg))
	r.POST("/upload/large", func(c *zen.Ctx) {
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
	r.POST("/upload", func(c *zen.Ctx) {
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
	r.GET("/panic", func(c *zen.Ctx) {
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
	r.GET("/log", func(c *zen.Ctx) {
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

	c := &zen.Ctx{Request: req}
	got := c.ClientIP()
	if got != "1.2.3.4" {
		t.Fatalf("ip = %q, want %q", got, "1.2.3.4")
	}
}

func TestClientIP_XRealIP(t *testing.T) {
	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("X-Real-IP", "10.0.0.1")
	req.RemoteAddr = "127.0.0.1:12345"

	c := &zen.Ctx{Request: req}
	got := c.ClientIP()
	if got != "10.0.0.1" {
		t.Fatalf("ip = %q, want %q", got, "10.0.0.1")
	}
}

func TestClientIP_RemoteAddr(t *testing.T) {
	req := httptest.NewRequest("GET", "/", nil)
	req.RemoteAddr = "192.168.1.1:54321"

	c := &zen.Ctx{Request: req}
	got := c.ClientIP()
	if got != "192.168.1.1" {
		t.Fatalf("ip = %q, want %q", got, "192.168.1.1")
	}
}

// Test Compress middleware
func TestCompress_GzipResponse(t *testing.T) {
	r := zen.New(":0")
	r.Use(Compress())
	r.GET("/api", func(c *zen.Ctx) {
		c.String(200, strings.Repeat("x", 2048))
	})

	req := httptest.NewRequest("GET", "/api", nil)
	req.Header.Set("Accept-Encoding", "gzip")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("status = %d, want 200", w.Code)
	}
	if w.Header().Get("Content-Encoding") != "gzip" {
		t.Fatalf("Content-Encoding = %q, want gzip", w.Header().Get("Content-Encoding"))
	}
}

func TestCompress_NoGzip_Skip(t *testing.T) {
	r := zen.New(":0")
	r.Use(Compress())
	r.GET("/api", func(c *zen.Ctx) {
		c.String(200, strings.Repeat("x", 2048))
	})

	req := httptest.NewRequest("GET", "/api", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("status = %d, want 200", w.Code)
	}
	if w.Header().Get("Content-Encoding") != "" {
		t.Fatalf("Content-Encoding should not be set")
	}
}

func TestCompress_Level(t *testing.T) {
	r := zen.New(":0")
	r.Use(CompressWithLevel(gzip.BestSpeed))
	r.GET("/api", func(c *zen.Ctx) {
		c.String(200, strings.Repeat("x", 2048))
	})

	req := httptest.NewRequest("GET", "/api", nil)
	req.Header.Set("Accept-Encoding", "gzip")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("status = %d, want 200", w.Code)
	}
	if w.Header().Get("Content-Encoding") != "gzip" {
		t.Fatal("Content-Encoding should be gzip")
	}
}

// A panic with Recover + Compress must produce a real 500, not an empty 200.
func TestCompress_RecoverPanic(t *testing.T) {
	r := zen.New(":0")
	r.Use(Recover, Compress())
	r.GET("/boom", func(c *zen.Ctx) {
		panic("kaboom")
	})

	req := httptest.NewRequest("GET", "/boom", nil)
	req.Header.Set("Accept-Encoding", "gzip")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != 500 {
		t.Fatalf("status = %d, want 500", w.Code)
	}
	if !strings.Contains(w.Body.String(), "Internal Server Error") {
		t.Fatalf("body = %q, want it to contain the 500 message", w.Body.String())
	}
}

// The first WriteHeader must win, matching the http.ResponseWriter contract.
func TestCompress_WriteHeaderFirstWins(t *testing.T) {
	r := zen.New(":0")
	r.Use(Compress())
	r.GET("/twice", func(c *zen.Ctx) {
		c.Response.WriteHeader(404)
		c.Response.WriteHeader(200)
		_, _ = c.Response.Write([]byte("body"))
	})

	req := httptest.NewRequest("GET", "/twice", nil)
	req.Header.Set("Accept-Encoding", "gzip")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != 404 {
		t.Fatalf("status = %d, want 404 (first WriteHeader must win)", w.Code)
	}
	if w.Body.String() != "body" {
		t.Fatalf("body = %q, want %q", w.Body.String(), "body")
	}
}

// Small SSE events must be delivered on Flush even when below the 1 KB
// compression threshold, not withheld until the response completes.
func TestCompress_SSEStreams(t *testing.T) {
	r := zen.New(":0")
	r.Use(Compress())
	r.GET("/events", func(c *zen.Ctx) {
		_ = c.SSEvent("msg", "first")
		_ = c.SSEvent("msg", "second")
	})

	req := httptest.NewRequest("GET", "/events", nil)
	req.Header.Set("Accept-Encoding", "gzip")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("status = %d, want 200", w.Code)
	}
	if w.Header().Get("Content-Encoding") == "gzip" {
		t.Fatal("streaming response should not be gzip compressed")
	}
	body := w.Body.String()
	if !strings.Contains(body, "event: msg") || !strings.Contains(body, "first") || !strings.Contains(body, "second") {
		t.Fatalf("body = %q, want both SSE events delivered", body)
	}
}

// Compress must append to an existing Vary header, not overwrite it.
func TestCompress_VaryAppend(t *testing.T) {
	r := zen.New(":0")
	r.Use(Compress())
	r.GET("/api", func(c *zen.Ctx) {
		c.Response.Header().Set("Vary", "Origin")
		c.String(200, strings.Repeat("x", 2048))
	})

	req := httptest.NewRequest("GET", "/api", nil)
	req.Header.Set("Accept-Encoding", "gzip")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	vals := w.Header().Values("Vary")
	if !slices.Contains(vals, "Origin") || !slices.Contains(vals, "Accept-Encoding") {
		t.Fatalf("Vary = %v, want both Origin and Accept-Encoding", vals)
	}
}

// A handler that only sets a status without a body (e.g. 204/304) must not be
// rewritten to 200 by the deferred compression flush.
func TestCompress_StatusOnly(t *testing.T) {
	for _, status := range []int{204, 304} {
		r := zen.New(":0")
		r.Use(Compress())
		r.GET("/status", func(c *zen.Ctx) {
			c.Status(status)
		})

		req := httptest.NewRequest("GET", "/status", nil)
		req.Header.Set("Accept-Encoding", "gzip")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if w.Code != status {
			t.Fatalf("status = %d, want %d", w.Code, status)
		}
	}
}

func TestLogger_SingleWriteHeader(t *testing.T) {
	r := zen.New(":0")
	r.Use(Logger)
	r.GET("/twice", func(c *zen.Ctx) {
		c.Response.WriteHeader(404)
		c.Response.WriteHeader(200)
	})

	req := httptest.NewRequest("GET", "/twice", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != 404 {
		t.Fatalf("status = %d, want 404 (first WriteHeader must win)", w.Code)
	}
}

func TestRateLimiter_NegativeLimit(t *testing.T) {
	cfg := DefaultRateLimiterConfig()
	cfg.Limit = -5
	cfg.Duration = time.Hour

	r := zen.New(":0")
	r.Use(RateLimiterWithConfig(cfg))
	r.GET("/api", func(c *zen.Ctx) {
		c.String(200, "ok")
	})

	req := httptest.NewRequest("GET", "/api", nil)
	req.RemoteAddr = "127.0.0.1:12345"
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("status = %d, want 200", w.Code)
	}
}

func TestTimeout_Enforces504(t *testing.T) {
	r := zen.New(":0")
	r.Use(Timeout(50 * time.Millisecond))
	r.GET("/slow", func(c *zen.Ctx) {
		time.Sleep(200 * time.Millisecond)
		c.String(200, "too late")
	})

	req := httptest.NewRequest("GET", "/slow", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != 504 {
		t.Fatalf("status = %d, want 504", w.Code)
	}
	if w.Body.String() != "" {
		t.Fatalf("body = %q, want empty (late writes must be discarded)", w.Body.String())
	}
}

func TestTimeout_WriteAfterTimeoutDiscarded(t *testing.T) {
	r := zen.New(":0")
	r.Use(Timeout(50 * time.Millisecond))
	r.GET("/slow", func(c *zen.Ctx) {
		time.Sleep(200 * time.Millisecond)
		_, _ = c.Response.Write([]byte("late"))
	})

	req := httptest.NewRequest("GET", "/slow", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != 504 {
		t.Fatalf("status = %d, want 504", w.Code)
	}
	if strings.Contains(w.Body.String(), "late") {
		t.Fatalf("body = %q, late write should have been discarded", w.Body.String())
	}
}

// Test RateLimiter middleware
func TestRateLimiter_Allow(t *testing.T) {
	r := zen.New(":0")
	r.Use(RateLimiter())
	r.GET("/api", func(c *zen.Ctx) {
		c.String(200, "ok")
	})

	req := httptest.NewRequest("GET", "/api", nil)
	req.RemoteAddr = "127.0.0.1:12345"
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("status = %d, want 200", w.Code)
	}
}

func TestRateLimiter_Exceed(t *testing.T) {
	cfg := DefaultRateLimiterConfig()
	cfg.Limit = 2
	cfg.Duration = time.Hour

	r := zen.New(":0")
	r.Use(RateLimiterWithConfig(cfg))
	r.GET("/api", func(c *zen.Ctx) {
		c.String(200, "ok")
	})

	// Make 3 requests (exceeds limit of 2)
	for i := 0; i < 3; i++ {
		req := httptest.NewRequest("GET", "/api", nil)
		req.RemoteAddr = "10.0.0.1:12345"
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if i < 2 {
			if w.Code != 200 {
				t.Fatalf("request %d: status = %d, want 200", i, w.Code)
			}
		} else {
			if w.Code != 429 {
				t.Fatalf("request %d: status = %d, want 429", i, w.Code)
			}
		}
	}
}

func TestRateLimiter_CustomKeyFunc(t *testing.T) {
	cfg := DefaultRateLimiterConfig()
	cfg.Limit = 1
	cfg.Duration = time.Hour
	cfg.KeyFunc = func(r *http.Request) string {
		return r.Header.Get("X-API-Key")
	}

	r := zen.New(":0")
	r.Use(RateLimiterWithConfig(cfg))
	r.GET("/api", func(c *zen.Ctx) {
		c.String(200, "ok")
	})

	req := httptest.NewRequest("GET", "/api", nil)
	req.Header.Set("X-API-Key", "key-1")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != 200 {
		t.Fatalf("first request: status = %d, want 200", w.Code)
	}

	// Different key should be allowed
	req2 := httptest.NewRequest("GET", "/api", nil)
	req2.Header.Set("X-API-Key", "key-2")
	w2 := httptest.NewRecorder()
	r.ServeHTTP(w2, req2)
	if w2.Code != 200 {
		t.Fatalf("second request (different key): status = %d, want 200", w2.Code)
	}

	// Same key as first should be blocked
	req3 := httptest.NewRequest("GET", "/api", nil)
	req3.Header.Set("X-API-Key", "key-1")
	w3 := httptest.NewRecorder()
	r.ServeHTTP(w3, req3)
	if w3.Code != 429 {
		t.Fatalf("third request (same key): status = %d, want 429", w3.Code)
	}
}

func TestRateLimiter_Skipper(t *testing.T) {
	cfg := DefaultRateLimiterConfig()
	cfg.Limit = 1
	cfg.Duration = time.Hour
	cfg.Skipper = func(r *http.Request) bool {
		return r.URL.Path == "/public"
	}

	r := zen.New(":0")
	r.Use(RateLimiterWithConfig(cfg))
	var callCount int
	r.GET("/public", func(c *zen.Ctx) {
		callCount++
		c.String(200, "ok")
	})
	r.GET("/private", func(c *zen.Ctx) {
		callCount++
		c.String(200, "ok")
	})

	// Skipped route: unlimited
	for i := 0; i < 5; i++ {
		req := httptest.NewRequest("GET", "/public", nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != 200 {
			t.Fatalf("request %d to /public: status = %d, want 200", i, w.Code)
		}
	}

	// Non-skipped route: limited
	req := httptest.NewRequest("GET", "/private", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != 200 {
		t.Fatalf("first request to /private: status = %d, want 200", w.Code)
	}

	req2 := httptest.NewRequest("GET", "/private", nil)
	w2 := httptest.NewRecorder()
	r.ServeHTTP(w2, req2)
	if w2.Code != 429 {
		t.Fatalf("second request to /private: status = %d, want 429", w2.Code)
	}
}

func TestRateLimiter_Headers(t *testing.T) {
	cfg := DefaultRateLimiterConfig()
	cfg.Limit = 5
	cfg.Duration = time.Hour

	r := zen.New(":0")
	r.Use(RateLimiterWithConfig(cfg))
	r.GET("/api", func(c *zen.Ctx) {
		c.String(200, "ok")
	})

	req := httptest.NewRequest("GET", "/api", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("status = %d, want 200", w.Code)
	}
	if w.Header().Get("X-RateLimit-Limit") != "5" {
		t.Fatalf("X-RateLimit-Limit = %q, want %q", w.Header().Get("X-RateLimit-Limit"), "5")
	}
}

func TestPprof_Handler(t *testing.T) {
	r := zen.New(":0")
	RegisterPprof(r)

	req := httptest.NewRequest("GET", "/debug/pprof/", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("status = %d, want 200", w.Code)
	}
}

func TestRequestID_Default(t *testing.T) {
	r := zen.New(":0")
	r.Use(RequestID())
	r.GET("/", func(c *zen.Ctx) {
		id := GetRequestID(c)
		if id == "" {
			t.Fatal("expected non-empty request ID")
		}
		c.String(200, id)
	})

	req := httptest.NewRequest("GET", "/", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("status = %d, want 200", w.Code)
	}
	if w.Header().Get(zen.HeaderXRequestID) == "" {
		t.Fatal("expected X-Request-Id header")
	}
	if w.Body.String() != w.Header().Get(zen.HeaderXRequestID) {
		t.Fatal("response body should match request ID")
	}
}

// TestRequestID_HeaderConstant pins the middleware output to the exported
// zen.HeaderXRequestID constant so a rename on either side fails here.
func TestRequestID_HeaderConstant(t *testing.T) {
	r := zen.New(":0")
	r.Use(RequestID())
	r.GET("/", func(c *zen.Ctx) {
		c.String(200, "ok")
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/", nil)
	r.ServeHTTP(w, req)

	if got := w.Header().Get(zen.HeaderXRequestID); got == "" {
		t.Fatalf("expected non-empty %q response header", zen.HeaderXRequestID)
	}
}

func TestRequestID_ClientProvided(t *testing.T) {
	r := zen.New(":0")
	r.Use(RequestID())
	r.GET("/", func(c *zen.Ctx) {
		c.String(200, GetRequestID(c))
	})

	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("X-Request-ID", "client-id-123")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Body.String() != "client-id-123" {
		t.Fatalf("body = %q, want %q", w.Body.String(), "client-id-123")
	}
}

func TestRequestID_CustomHeader(t *testing.T) {
	cfg := DefaultRequestIDConfig()
	cfg.Header = "X-Trace-ID"
	cfg.Generator = func() string { return "trace-42" }

	r := zen.New(":0")
	r.Use(RequestIDWithConfig(cfg))
	r.GET("/", func(c *zen.Ctx) {
		c.String(200, GetRequestID(c))
	})

	req := httptest.NewRequest("GET", "/", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Header().Get("X-Trace-Id") != "trace-42" {
		t.Fatalf("X-Trace-Id = %q, want %q", w.Header().Get("X-Trace-Id"), "trace-42")
	}
}

func TestTimeout_Default(t *testing.T) {
	r := zen.New(":0")
	r.Use(Timeout(5 * time.Second))
	r.GET("/", func(c *zen.Ctx) {
		c.String(200, "ok")
	})

	req := httptest.NewRequest("GET", "/", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("status = %d, want 200", w.Code)
	}
	if w.Body.String() != "ok" {
		t.Fatalf("body = %q, want %q", w.Body.String(), "ok")
	}
}

func TestTimeout_ContextDeadline(t *testing.T) {
	r := zen.New(":0")
	r.Use(Timeout(time.Hour))
	r.GET("/", func(c *zen.Ctx) {
		deadline, ok := c.Request.Context().Deadline()
		if !ok {
			t.Fatal("expected deadline on context")
		}
		if time.Until(deadline) < 50*time.Minute {
			t.Fatal("deadline too soon")
		}
		c.String(200, "ok")
	})

	req := httptest.NewRequest("GET", "/", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("status = %d, want 200", w.Code)
	}
}

func TestTimeout_Skipped(t *testing.T) {
	cfg := DefaultTimeoutConfig()
	cfg.Duration = 30 * time.Second
	cfg.Skipper = func(r *http.Request) bool {
		return r.URL.Path == "/skip"
	}

	r := zen.New(":0")
	r.Use(TimeoutWithConfig(cfg))
	r.GET("/skip", func(c *zen.Ctx) {
		_, ok := c.Request.Context().Deadline()
		if ok {
			t.Fatal("expected no deadline for skipped request")
		}
		c.String(200, "ok")
	})

	req := httptest.NewRequest("GET", "/skip", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("status = %d, want 200", w.Code)
	}
}

func TestRequestID_Skipper(t *testing.T) {
	cfg := DefaultRequestIDConfig()
	cfg.Skipper = func(r *http.Request) bool {
		return r.URL.Path == "/skip"
	}

	r := zen.New(":0")
	r.Use(RequestIDWithConfig(cfg))
	r.GET("/skip", func(c *zen.Ctx) {
		c.String(200, GetRequestID(c))
	})

	req := httptest.NewRequest("GET", "/skip", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Header().Get("X-Request-ID") != "" {
		t.Fatal("expected no X-Request-ID for skipped request")
	}
}
