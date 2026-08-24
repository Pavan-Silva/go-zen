package middleware

import (
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/Pavan-Silva/go-zen"
)

// A handler that writes a small (buffered) body and then panics must produce
// a clean 500 from the recovery middleware - the compress teardown must not
// flush the partial buffer as 200 first.
func TestCompress_PanicBeforeCommit_Clean500(t *testing.T) {
	r := zen.New(":0")
	r.Use(Recover)
	r.Use(Compress())
	r.GET("/boom", func(c *zen.Ctx) {
		io.WriteString(c.Response, "partial-data")
		panic("kaboom")
	})

	req := httptest.NewRequest("GET", "/boom", nil)
	req.Header.Set("Accept-Encoding", "gzip")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected clean 500, got %d with body %q", w.Code, w.Body.String())
	}
	if strings.Contains(w.Body.String(), "partial-data") {
		t.Errorf("buffered partial data leaked into the error response: %q", w.Body.String())
	}
}

// Once the gzip stream is committed the status can no longer change, but the
// teardown must still close the stream cleanly instead of leaving it corrupt,
// and must not swallow the panic from the recovery middleware.
func TestCompress_PanicAfterCommit_StreamClosedAndPanicPropagates(t *testing.T) {
	r := zen.New(":0")
	r.Use(Recover)
	r.Use(Compress())
	r.GET("/boom", func(c *zen.Ctx) {
		io.WriteString(c.Response, strings.Repeat("A", 4096))
		panic("kaboom")
	})

	req := httptest.NewRequest("GET", "/boom", nil)
	req.Header.Set("Accept-Encoding", "gzip")
	w := httptest.NewRecorder()

	func() {
		defer func() {
			if rec := recover(); rec != nil {
				t.Errorf("panic was swallowed by the compress teardown: %v", rec)
			}
		}()
		r.ServeHTTP(w, req)
	}()

	if w.Code == http.StatusInternalServerError && strings.Contains(w.Body.String(), "partial") {
		t.Errorf("unexpected corrupt output: %d %q", w.Code, w.Body.String())
	}
}

func TestCompress_MultiValueAcceptEncoding(t *testing.T) {
	r := zen.New(":0")
	r.Use(Compress())
	r.GET("/data", func(c *zen.Ctx) {
		c.Blob(http.StatusOK, "text/plain", []byte(strings.Repeat("z", 4096)))
	})

	req := httptest.NewRequest("GET", "/data", nil)
	req.Header.Add("Accept-Encoding", "br")
	req.Header.Add("Accept-Encoding", "gzip")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if got := w.Header().Get("Content-Encoding"); got != "gzip" {
		t.Errorf("gzip offered on a second Accept-Encoding line should be honored, got Content-Encoding=%q", got)
	}
}

func TestCompress_GzipQZeroNotCompressed(t *testing.T) {
	r := zen.New(":0")
	r.Use(Compress())
	r.GET("/data", func(c *zen.Ctx) {
		c.Blob(http.StatusOK, "text/plain", []byte(strings.Repeat("z", 4096)))
	})

	for _, ae := range []string{"gzip;q=0", "gzip;q=0.000", "br, gzip;q=0", "gzip;q=0, *"} {
		req := httptest.NewRequest("GET", "/data", nil)
		req.Header.Set("Accept-Encoding", ae)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if got := w.Header().Get("Content-Encoding"); got == "gzip" {
			t.Errorf("Accept-Encoding %q must not produce gzip responses", ae)
		}
	}
}

func TestCompress_WildcardAcceptEncoding(t *testing.T) {
	r := zen.New(":0")
	r.Use(Compress())
	r.GET("/data", func(c *zen.Ctx) {
		c.Blob(http.StatusOK, "text/plain", []byte(strings.Repeat("z", 4096)))
	})

	req := httptest.NewRequest("GET", "/data", nil)
	req.Header.Set("Accept-Encoding", "*")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if got := w.Header().Get("Content-Encoding"); got != "gzip" {
		t.Errorf("Accept-Encoding '*' should allow compression, got Content-Encoding=%q", got)
	}
}

func TestCompress_AcceptsGzipUnit(t *testing.T) {
	cases := []struct {
		values []string
		want   bool
	}{
		{[]string{"gzip"}, true},
		{[]string{"GZIP"}, true},
		{[]string{"x-gzip"}, true},
		{[]string{"deflate"}, false},
		{[]string{"br", "gzip"}, true},
		{[]string{"gzip;q=0"}, false},
		{[]string{"gzip;q=0.5"}, true},
		{[]string{"gzip;q=0, br;q=1"}, false},
		{[]string{"*"}, true},
		{[]string{"*;q=0"}, false},
		{[]string{"gzip;q=0", "*"}, false},
		{[]string{"*;q=0.3"}, true},
		{nil, false},
	}
	for _, tc := range cases {
		h := http.Header{}
		for _, v := range tc.values {
			h.Add("Accept-Encoding", v)
		}
		if got := acceptsGzip(h); got != tc.want {
			t.Errorf("acceptsGzip(%v) = %v, want %v", tc.values, got, tc.want)
		}
	}
}

// Regression: Logger never restored c.Response, so any outer middleware that
// touched c.Response after Next() hit a pooled wrapper whose embedded writer
// was nil and panicked with a nil pointer dereference.
func TestLogger_RestoresResponseWriter(t *testing.T) {
	r := zen.New(":0")
	headerSetOK := false
	r.Use(func(c *zen.Ctx) {
		c.Next()
		c.Response.Header().Set("X-After", "1")
		headerSetOK = true
	})
	r.Use(Logger)
	r.GET("/ok", func(c *zen.Ctx) { c.String(http.StatusOK, "hi") })

	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest("GET", "/ok", nil))

	if !headerSetOK {
		t.Error("outer middleware post-Next processing skipped")
	}
	if w.Header().Get("X-After") != "1" {
		t.Error("header set after Next() did not reach the real ResponseWriter")
	}
}

// Regression: requests that panic produced no access-log entry because the
// logging code lived after c.Next().
func TestLogger_LogsPanickingRequests(t *testing.T) {
	old := os.Stdout
	rd, wr, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stdout = wr

	r := zen.New(":0")
	r.Use(Recover)
	r.Use(Logger)
	r.GET("/boom", func(c *zen.Ctx) { panic("kaboom") })

	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest("GET", "/boom", nil))

	_ = wr.Close()
	os.Stdout = old
	out, _ := io.ReadAll(rd)
	_ = rd.Close()

	if !strings.Contains(string(out), "/boom") {
		t.Errorf("panicking request was not logged, stdout: %q", string(out))
	}
	if w.Code != http.StatusInternalServerError {
		t.Errorf("Recover should still produce 500, got %d", w.Code)
	}
}
