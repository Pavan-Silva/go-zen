package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Pavan-Silva/go-zen"
)

func TestCrossOriginProtection_SafeMethods(t *testing.T) {
	methods := []string{"GET", "HEAD", "OPTIONS"}
	for _, method := range methods {
		r := zen.New(":0")
		r.Use(CrossOriginProtection())
		r.Handle(method+" /api", func(c *zen.Context) { c.String(200, "ok") })

		req := httptest.NewRequest(method, "/api", nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if w.Code != 200 {
			t.Fatalf("method %s: status = %d, want 200", method, w.Code)
		}
	}
}

func TestCrossOriginProtection_UnsafeMethodsNoHeaders(t *testing.T) {
	methods := []string{"POST", "PUT", "DELETE", "PATCH"}
	for _, method := range methods {
		r := zen.New(":0")
		r.Use(CrossOriginProtection())
		r.Handle(method+" /api", func(c *zen.Context) { c.String(200, "ok") })

		req := httptest.NewRequest(method, "/api", nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		// No Sec-Fetch-Site, no Origin → allowed (assumed same-origin or non-browser)
		if w.Code != 200 {
			t.Fatalf("method %s: status = %d, want 200", method, w.Code)
		}
	}
}

func TestCrossOriginProtection_SecFetchSiteSameOrigin(t *testing.T) {
	r := zen.New(":0")
	r.Use(CrossOriginProtection())
	r.Handle("POST /api", func(c *zen.Context) { c.String(200, "ok") })

	req := httptest.NewRequest("POST", "/api", nil)
	req.Header.Set("Sec-Fetch-Site", "same-origin")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("status = %d, want 200", w.Code)
	}
}

func TestCrossOriginProtection_SecFetchSiteNone(t *testing.T) {
	r := zen.New(":0")
	r.Use(CrossOriginProtection())
	r.Handle("POST /api", func(c *zen.Context) { c.String(200, "ok") })

	req := httptest.NewRequest("POST", "/api", nil)
	req.Header.Set("Sec-Fetch-Site", "none")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("status = %d, want 200", w.Code)
	}
}

func TestCrossOriginProtection_SecFetchSiteCrossSite(t *testing.T) {
	r := zen.New(":0")
	r.Use(CrossOriginProtection())
	r.Handle("POST /api", func(c *zen.Context) { c.String(200, "ok") })

	req := httptest.NewRequest("POST", "/api", nil)
	req.Header.Set("Sec-Fetch-Site", "cross-site")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != 403 {
		t.Fatalf("status = %d, want 403", w.Code)
	}
}

func TestCrossOriginProtection_SecFetchSiteSameSite(t *testing.T) {
	r := zen.New(":0")
	r.Use(CrossOriginProtection())
	r.Handle("POST /api", func(c *zen.Context) { c.String(200, "ok") })

	req := httptest.NewRequest("POST", "/api", nil)
	req.Header.Set("Sec-Fetch-Site", "same-site")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != 403 {
		t.Fatalf("status = %d, want 403", w.Code)
	}
}

func TestCrossOriginProtection_OriginMatch(t *testing.T) {
	r := zen.New(":0")
	r.Use(CrossOriginProtection())
	r.Handle("POST /api", func(c *zen.Context) { c.String(200, "ok") })

	req := httptest.NewRequest("POST", "/api", nil)
	req.Host = "example.com"
	req.Header.Set("Origin", "http://example.com")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("status = %d, want 200", w.Code)
	}
}

func TestCrossOriginProtection_OriginMismatch(t *testing.T) {
	r := zen.New(":0")
	r.Use(CrossOriginProtection())
	r.Handle("POST /api", func(c *zen.Context) { c.String(200, "ok") })

	req := httptest.NewRequest("POST", "/api", nil)
	req.Host = "example.com"
	req.Header.Set("Origin", "http://evil.com")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != 403 {
		t.Fatalf("status = %d, want 403", w.Code)
	}
}

func TestCrossOriginProtection_TrustedOrigin(t *testing.T) {
	cfg := DefaultCrossOriginProtectionConfig()
	cfg.TrustedOrigins = []string{"https://trusted.example.com"}
	r := zen.New(":0")
	r.Use(CrossOriginProtectionWithConfig(cfg))
	r.Handle("POST /api", func(c *zen.Context) { c.String(200, "ok") })

	req := httptest.NewRequest("POST", "/api", nil)
	req.Host = "app.example.com"
	req.Header.Set("Sec-Fetch-Site", "cross-site")
	req.Header.Set("Origin", "https://trusted.example.com")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("status = %d, want 200", w.Code)
	}
}

func TestCrossOriginProtection_BypassPattern(t *testing.T) {
	cfg := DefaultCrossOriginProtectionConfig()
	cfg.InsecureBypassPatterns = []string{"POST /api"}
	r := zen.New(":0")
	r.Use(CrossOriginProtectionWithConfig(cfg))
	r.Handle("POST /api", func(c *zen.Context) { c.String(200, "ok") })

	req := httptest.NewRequest("POST", "/api", nil)
	req.Header.Set("Sec-Fetch-Site", "cross-site")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("status = %d, want 200", w.Code)
	}
}

func TestCrossOriginProtection_DenyHandler(t *testing.T) {
	cfg := DefaultCrossOriginProtectionConfig()
	cfg.DenyHandler = func(c *zen.Context) {
		c.String(400, "custom csrf error")
	}
	r := zen.New(":0")
	r.Use(CrossOriginProtectionWithConfig(cfg))
	r.Handle("POST /api", func(c *zen.Context) { c.String(200, "ok") })

	req := httptest.NewRequest("POST", "/api", nil)
	req.Header.Set("Sec-Fetch-Site", "cross-site")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != 400 {
		t.Fatalf("status = %d, want 400", w.Code)
	}
	if w.Body.String() != "custom csrf error" {
		t.Fatalf("body = %q, want %q", w.Body.String(), "custom csrf error")
	}
}

func TestCrossOriginProtection_Skipper(t *testing.T) {
	cfg := DefaultCrossOriginProtectionConfig()
	cfg.Skipper = func(r *http.Request) bool {
		return r.URL.Path == "/skip"
	}
	r := zen.New(":0")
	r.Use(CrossOriginProtectionWithConfig(cfg))
	r.Handle("POST /skip", func(c *zen.Context) { c.String(200, "ok") })

	req := httptest.NewRequest("POST", "/skip", nil)
	req.Header.Set("Sec-Fetch-Site", "cross-site")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("status = %d, want 200", w.Code)
	}
}

func TestCrossOriginProtection_DefaultConfig(t *testing.T) {
	cfg := DefaultCrossOriginProtectionConfig()
	if cfg.TrustedOrigins != nil {
		t.Fatal("TrustedOrigins should be nil by default")
	}
	if cfg.InsecureBypassPatterns != nil {
		t.Fatal("InsecureBypassPatterns should be nil by default")
	}
	if cfg.DenyHandler != nil {
		t.Fatal("DenyHandler should be nil by default")
	}
	if cfg.Skipper != nil {
		t.Fatal("Skipper should be nil by default")
	}
}

func TestCrossOriginProtection_SafeMethodWithCrossSite(t *testing.T) {
	r := zen.New(":0")
	r.Use(CrossOriginProtection())
	r.Handle("GET /api", func(c *zen.Context) { c.String(200, "ok") })

	req := httptest.NewRequest("GET", "/api", nil)
	req.Header.Set("Sec-Fetch-Site", "cross-site")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	// Safe methods are always allowed regardless of Sec-Fetch-Site
	if w.Code != 200 {
		t.Fatalf("status = %d, want 200", w.Code)
	}
}
