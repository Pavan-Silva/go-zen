package zen

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

// Test Header binding
func TestBindHeader(t *testing.T) {
	type Headers struct {
		UserID string `header:"X-User-Id"`
		APIKey string `header:"X-Api-Key"`
		Rate   int    `header:"X-Rate-Limit"`
	}

	r := New(":0")
	r.GET("/headers", func(c *Ctx) {
		var h Headers
		if err := BindHeaders(c, &h); err != nil {
			c.Error(http.StatusBadRequest, err.Error())
			return
		}
		c.JSON(http.StatusOK, h)
	})

	req := httptest.NewRequest("GET", "/headers", nil)
	req.Header.Set("X-User-Id", "user-123")
	req.Header.Set("X-Api-Key", "secret-key")
	req.Header.Set("X-Rate-Limit", "100")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("status = %d, want 200", w.Code)
	}
	if w.Body.String() == "" {
		t.Fatal("body should not be empty")
	}
}

func TestBindHeader_NotFound(t *testing.T) {
	type Headers struct {
		UserID string `header:"X-User-Id"`
	}

	r := New(":0")
	r.GET("/headers", func(c *Ctx) {
		var h Headers
		if err := BindHeaders(c, &h); err != nil {
			c.Error(http.StatusBadRequest, err.Error())
			return
		}
		c.JSON(http.StatusOK, h)
	})

	req := httptest.NewRequest("GET", "/headers", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("status = %d, want 200", w.Code)
	}
}

func TestBindHeader_InvalidDest(t *testing.T) {
	r := New(":0")
	r.GET("/headers", func(c *Ctx) {
		// Pass a non-pointer (invalid dest)
		var h struct{}
		if err := BindHeaders(c, h); err != nil {
			c.Error(http.StatusBadRequest, err.Error())
			return
		}
		c.String(200, "ok")
	})

	req := httptest.NewRequest("GET", "/headers", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != 400 {
		t.Fatalf("status = %d, want 400", w.Code)
	}
}
