package middleware

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

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
	// Default config has no allowed origins, so CORS headers should not be set
	if w.Header().Get("Access-Control-Allow-Origin") != "" {
		t.Fatalf("ACAO should not be set for default config")
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
	cfg := DefaultCORSConfig()
	cfg.AllowedOrigins = []string{"https://example.com"}
	
	r := zen.New(":0")
	r.Use(CORS(cfg))
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
	cfg.AllowedOrigins = []string{"https://example.com"}
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

// Test Compress middleware
func TestCompress_GzipResponse(t *testing.T) {
	r := zen.New(":0")
	r.Use(Compress())
	r.Handle("GET /api", func(c *zen.Context) {
		c.String(200, "hello world")
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
	r.Handle("GET /api", func(c *zen.Context) {
		c.String(200, "hello world")
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

func TestCompress_Skipper(t *testing.T) {
	r := zen.New(":0")
	r.Use(CompressWithSkipper(func(r *http.Request) bool {
		return r.URL.Path == "/no-compress"
	}))
	r.Handle("GET /no-compress", func(c *zen.Context) {
		c.String(200, "hello")
	})

	req := httptest.NewRequest("GET", "/no-compress", nil)
	req.Header.Set("Accept-Encoding", "gzip")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Header().Get("Content-Encoding") != "" {
		t.Fatal("Content-Encoding should not be set when skipped")
	}
}

// Test RequestID middleware
func TestRequestID_Default(t *testing.T) {
	r := zen.New(":0")
	r.Use(RequestID())
	r.Handle("GET /api", func(c *zen.Context) {
		if val, ok := c.Get("request_id"); ok {
			c.String(200, val.(string))
		}
	})

	req := httptest.NewRequest("GET", "/api", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("status = %d, want 200", w.Code)
	}
	if w.Header().Get("X-Request-ID") == "" {
		t.Fatal("X-Request-ID header should be set")
	}
	if w.Body.String() != w.Header().Get("X-Request-ID") {
		t.Fatalf("context id != header id")
	}
}

func TestRequestID_ReuseExisting(t *testing.T) {
	r := zen.New(":0")
	r.Use(RequestID())
	r.Handle("GET /api", func(c *zen.Context) {
		if val, ok := c.Get("request_id"); ok {
			c.String(200, val.(string))
		}
	})

	req := httptest.NewRequest("GET", "/api", nil)
	req.Header.Set("X-Request-ID", "existing-id-123")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Body.String() != "existing-id-123" {
		t.Fatalf("body = %q, want %q", w.Body.String(), "existing-id-123")
	}
}

func TestRequestID_CustomHeader(t *testing.T) {
	cfg := DefaultRequestIDConfig()
	cfg.Header = "X-Trace-ID"
	r := zen.New(":0")
	r.Use(RequestIDWithConfig(cfg))
	r.Handle("GET /api", func(c *zen.Context) {
		c.String(200, "ok")
	})

	req := httptest.NewRequest("GET", "/api", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Header().Get("X-Trace-ID") == "" {
		t.Fatal("X-Trace-ID header should be set")
	}
}

// Test RateLimiter middleware
func TestRateLimiter_Allow(t *testing.T) {
	r := zen.New(":0")
	r.Use(RateLimiter())
	r.Handle("GET /api", func(c *zen.Context) {
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
	r.Handle("GET /api", func(c *zen.Context) {
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

// Test Prometheus middleware
func TestPrometheus_MetricsEndpoint(t *testing.T) {
	r := zen.New(":0")
	r.Use(Prometheus())
	r.Handle("GET /api", func(c *zen.Context) {
		c.String(200, "ok")
	})
	r.HandleRaw("GET /metrics", PrometheusHandler())

	// Make a request to generate metrics
	req := httptest.NewRequest("GET", "/api", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("status = %d, want 200", w.Code)
	}

	// Check metrics endpoint
	req2 := httptest.NewRequest("GET", "/metrics", nil)
	w2 := httptest.NewRecorder()
	r.ServeHTTP(w2, req2)

	if w2.Code != 200 {
		t.Fatalf("metrics status = %d, want 200", w2.Code)
	}
	if !strings.Contains(w2.Body.String(), "zen_http_requests_total") {
		t.Fatal("metrics should contain zen_http_requests_total")
	}
}

// Test Pprof handler
func TestCSRF_SafeMethodSetsCookie(t *testing.T) {
	r := zen.New(":0")
	r.Use(CSRF())
	r.Handle("GET /api", func(c *zen.Context) {
		if tok, ok := c.Get("csrf_token"); !ok || tok == "" {
			t.Fatal("csrf_token should be set in context")
		}
		c.String(200, "ok")
	})

	req := httptest.NewRequest("GET", "/api", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("status = %d, want 200", w.Code)
	}
	cookies := w.Result().Cookies()
	var csrfCookie *http.Cookie
	for _, c := range cookies {
		if c.Name == "_csrf" {
			csrfCookie = c
			break
		}
	}
	if csrfCookie == nil {
		t.Fatal("csrf cookie not set")
	}
	if csrfCookie.Value == "" {
		t.Fatal("csrf cookie value is empty")
	}
}

func TestCSRF_ReusesExistingCookie(t *testing.T) {
	r := zen.New(":0")
	r.Use(CSRF())
	callCount := 0
	r.Handle("GET /api", func(c *zen.Context) {
		callCount++
		c.String(200, "ok")
	})

	// First request - sets cookie
	req1 := httptest.NewRequest("GET", "/api", nil)
	w1 := httptest.NewRecorder()
	r.ServeHTTP(w1, req1)

	cookies := w1.Result().Cookies()
	var csrfCookie *http.Cookie
	for _, c := range cookies {
		if c.Name == "_csrf" {
			csrfCookie = c
			break
		}
	}
	if csrfCookie == nil {
		t.Fatal("csrf cookie not set on first request")
	}

	// Second request with existing cookie - should reuse token
	req2 := httptest.NewRequest("GET", "/api", nil)
	req2.AddCookie(csrfCookie)
	w2 := httptest.NewRecorder()
	r.ServeHTTP(w2, req2)

	cookies2 := w2.Result().Cookies()
	var csrfCookie2 *http.Cookie
	for _, c := range cookies2 {
		if c.Name == "_csrf" {
			csrfCookie2 = c
			break
		}
	}
	if csrfCookie2 == nil {
		t.Fatal("csrf cookie not set on second request")
	}
	if csrfCookie2.Value != csrfCookie.Value {
		t.Fatalf("csrf token changed: old=%q, new=%q", csrfCookie.Value, csrfCookie2.Value)
	}
}

func TestCSRF_ValidTokenOnPost(t *testing.T) {
	r := zen.New(":0")
	r.Use(CSRFWithConfig(DefaultCSRFConfig()))
	r.Handle("POST /api", func(c *zen.Context) {
		c.String(200, "ok")
	})

	// First GET to set cookie
	req := httptest.NewRequest("GET", "/api", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	cookies := w.Result().Cookies()

	// POST with valid token
	req2 := httptest.NewRequest("POST", "/api", nil)
	for _, c := range cookies {
		req2.AddCookie(c)
	}
	// Read token from Set-Cookie
	var token string
	for _, c := range cookies {
		if c.Name == "_csrf" {
			token = c.Value
			break
		}
	}
	req2.Header.Set("X-CSRF-Token", token)
	w2 := httptest.NewRecorder()
	r.ServeHTTP(w2, req2)

	if w2.Code != 200 {
		t.Fatalf("status = %d, want 200", w2.Code)
	}
}

func TestCSRF_InvalidTokenOnPost(t *testing.T) {
	r := zen.New(":0")
	r.Use(CSRF())
	r.Handle("POST /api", func(c *zen.Context) {
		c.String(200, "ok")
	})

	// First GET to set cookie
	req := httptest.NewRequest("GET", "/api", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	cookies := w.Result().Cookies()

	// POST with wrong token
	req2 := httptest.NewRequest("POST", "/api", nil)
	for _, c := range cookies {
		req2.AddCookie(c)
	}
	req2.Header.Set("X-CSRF-Token", "wrong-token")
	w2 := httptest.NewRecorder()
	r.ServeHTTP(w2, req2)

	if w2.Code != 403 {
		t.Fatalf("status = %d, want 403", w2.Code)
	}
}

func TestCSRF_MissingTokenOnPost(t *testing.T) {
	r := zen.New(":0")
	r.Use(CSRF())
	r.Handle("POST /api", func(c *zen.Context) {
		c.String(200, "ok")
	})

	// First GET to set cookie
	req := httptest.NewRequest("GET", "/api", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	cookies := w.Result().Cookies()

	// POST without token header
	req2 := httptest.NewRequest("POST", "/api", nil)
	for _, c := range cookies {
		req2.AddCookie(c)
	}
	w2 := httptest.NewRecorder()
	r.ServeHTTP(w2, req2)

	if w2.Code != 403 {
		t.Fatalf("status = %d, want 403", w2.Code)
	}
}

func TestCSRF_MissingCookieOnPost(t *testing.T) {
	r := zen.New(":0")
	r.Use(CSRF())
	r.Handle("POST /api", func(c *zen.Context) {
		c.String(200, "ok")
	})

	req := httptest.NewRequest("POST", "/api", nil)
	req.Header.Set("X-CSRF-Token", "some-token")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != 403 {
		t.Fatalf("status = %d, want 403", w.Code)
	}
}

func TestCSRF_SafeMethods(t *testing.T) {
	r := zen.New(":0")
	r.Use(CSRF())
	r.Handle("GET /api", func(c *zen.Context) { c.String(200, "get") })
	r.Handle("HEAD /api", func(c *zen.Context) { c.String(200, "head") })
	r.Handle("OPTIONS /api", func(c *zen.Context) { c.String(200, "options") })
	r.Handle("TRACE /api", func(c *zen.Context) { c.String(200, "trace") })

	methods := []string{"GET", "HEAD", "OPTIONS", "TRACE"}
	for _, method := range methods {
		req := httptest.NewRequest(method, "/api", nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != 200 {
			t.Fatalf("method %s: status = %d, want 200", method, w.Code)
		}
		cookies := w.Result().Cookies()
		var found bool
		for _, c := range cookies {
			if c.Name == "_csrf" {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("method %s: csrf cookie not set", method)
		}
	}
}

func TestCSRF_CustomHeader(t *testing.T) {
	cfg := DefaultCSRFConfig()
	cfg.TokenLookup = "header:X-Custom-CSRF"
	r := zen.New(":0")
	r.Use(CSRFWithConfig(cfg))
	r.Handle("POST /api", func(c *zen.Context) { c.String(200, "ok") })

	req := httptest.NewRequest("GET", "/api", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	cookies := w.Result().Cookies()

	var token string
	for _, c := range cookies {
		if c.Name == "_csrf" {
			token = c.Value
			break
		}
	}

	req2 := httptest.NewRequest("POST", "/api", nil)
	for _, c := range cookies {
		req2.AddCookie(c)
	}
	req2.Header.Set("X-Custom-CSRF", token)
	w2 := httptest.NewRecorder()
	r.ServeHTTP(w2, req2)

	if w2.Code != 200 {
		t.Fatalf("status = %d, want 200", w2.Code)
	}
}

func TestCSRF_FormToken(t *testing.T) {
	cfg := DefaultCSRFConfig()
	cfg.TokenLookup = "form:_csrf"
	r := zen.New(":0")
	r.Use(CSRFWithConfig(cfg))
	r.Handle("POST /api", func(c *zen.Context) { c.String(200, "ok") })

	req := httptest.NewRequest("GET", "/api", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	cookies := w.Result().Cookies()

	var token string
	for _, c := range cookies {
		if c.Name == "_csrf" {
			token = c.Value
			break
		}
	}

	body := "_csrf=" + token
	req2 := httptest.NewRequest("POST", "/api", strings.NewReader(body))
	req2.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	for _, c := range cookies {
		req2.AddCookie(c)
	}
	w2 := httptest.NewRecorder()
	r.ServeHTTP(w2, req2)

	if w2.Code != 200 {
		t.Fatalf("status = %d, want 200 (token=%q)", w2.Code, token)
	}
}

func TestCSRF_CustomErrorHandler(t *testing.T) {
	cfg := DefaultCSRFConfig()
	cfg.ErrorHandler = func(c *zen.Context) {
		c.String(400, "custom csrf error")
	}
	r := zen.New(":0")
	r.Use(CSRFWithConfig(cfg))
	r.Handle("POST /api", func(c *zen.Context) { c.String(200, "ok") })

	req := httptest.NewRequest("POST", "/api", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != 400 {
		t.Fatalf("status = %d, want 400", w.Code)
	}
	if w.Body.String() != "custom csrf error" {
		t.Fatalf("body = %q, want %q", w.Body.String(), "custom csrf error")
	}
}

func TestCSRF_Skipper(t *testing.T) {
	cfg := DefaultCSRFConfig()
	cfg.Skipper = func(r *http.Request) bool {
		return r.URL.Path == "/skip"
	}
	r := zen.New(":0")
	r.Use(CSRFWithConfig(cfg))
	r.Handle("POST /skip", func(c *zen.Context) { c.String(200, "ok") })

	req := httptest.NewRequest("POST", "/skip", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("status = %d, want 200", w.Code)
	}
}

func TestPprof_Handler(t *testing.T) {
	r := zen.New(":0")
	r.HandleRaw("GET /debug/pprof/", PprofHandler())
	r.HandleRaw("GET /debug/pprof/{name}", PprofHandler())

	req := httptest.NewRequest("GET", "/debug/pprof/", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("status = %d, want 200", w.Code)
	}
}
