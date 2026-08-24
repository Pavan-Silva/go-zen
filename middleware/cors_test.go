package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Pavan-Silva/go-zen"
)

func corsTestEngine() (*zen.Engine, *bool) {
	r := zen.New(":0")
	cfg := DefaultCORSConfig()
	cfg.AllowedOrigins = []string{"http://good.example"}
	r.Use(CORS(cfg))

	called := false
	r.OPTIONS("/api/thing", func(c *zen.Ctx) { called = true; c.String(http.StatusOK, "options-ok") })
	return r, &called
}

// Regression: every OPTIONS request with an Origin header used to be treated
// as a preflight and answered 204, bypassing registered OPTIONS handlers even
// when the request was not a preflight.
func TestCORS_PlainOptionsReachesHandler(t *testing.T) {
	r, called := corsTestEngine()

	req := httptest.NewRequest("OPTIONS", "/api/thing", nil)
	req.Header.Set("Origin", "http://good.example")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if !*called {
		t.Fatalf("OPTIONS handler bypassed (status %d) for non-preflight request", w.Code)
	}
	if w.Code != http.StatusOK {
		t.Errorf("expected 200 from OPTIONS handler, got %d", w.Code)
	}
	if got := w.Header().Get("Access-Control-Allow-Origin"); got != "http://good.example" {
		t.Errorf("CORS response headers missing, got Allow-Origin=%q", got)
	}
}

func TestCORS_PreflightStillIntercepted(t *testing.T) {
	r, _ := corsTestEngine()

	req := httptest.NewRequest("OPTIONS", "/api/thing", nil)
	req.Header.Set("Origin", "http://good.example")
	req.Header.Set("Access-Control-Request-Method", "POST")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusNoContent {
		t.Errorf("preflight should be answered with 204, got %d", w.Code)
	}
	if got := w.Header().Get("Access-Control-Allow-Methods"); got == "" {
		t.Error("preflight response missing Access-Control-Allow-Methods")
	}
}

func TestCORS_OptionsWithoutOriginReachesHandler(t *testing.T) {
	r, called := corsTestEngine()

	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest("OPTIONS", "/api/thing", nil))

	if !*called || w.Code != http.StatusOK {
		t.Errorf("same-origin OPTIONS without Origin header must reach handler, called=%v status=%d", *called, w.Code)
	}
}
