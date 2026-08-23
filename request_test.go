package zen

import (
	"net/http/httptest"
	"testing"
)

func TestQueryParams(t *testing.T) {
	r := httptest.NewRequest("GET", "/search?q=golang&page=2&tags=go&tags=zen", nil)
	c := &Ctx{Request: r}

	qp := c.QueryParams()
	if len(qp) != 3 {
		t.Fatalf("QueryParams() len = %d, want 3", len(qp))
	}
	if qp["q"][0] != "golang" {
		t.Fatalf("q = %q, want %q", qp["q"][0], "golang")
	}
	if qp["page"][0] != "2" {
		t.Fatalf("page = %q, want %q", qp["page"][0], "2")
	}
	if len(qp["tags"]) != 2 || qp["tags"][0] != "go" || qp["tags"][1] != "zen" {
		t.Fatalf("tags = %v, want [go zen]", qp["tags"])
	}
}

func TestQueryParams_Empty(t *testing.T) {
	r := httptest.NewRequest("GET", "/", nil)
	c := &Ctx{Request: r}

	qp := c.QueryParams()
	if len(qp) != 0 {
		t.Fatalf("QueryParams() len = %d, want 0", len(qp))
	}
}

func TestDefaultQuery_Present(t *testing.T) {
	r := httptest.NewRequest("GET", "/items?page=3", nil)
	c := &Ctx{Request: r}

	if got := c.DefaultQuery("page", "1"); got != "3" {
		t.Fatalf("DefaultQuery = %q, want %q", got, "3")
	}
}

func TestDefaultQuery_Missing(t *testing.T) {
	r := httptest.NewRequest("GET", "/items", nil)
	c := &Ctx{Request: r}

	if got := c.DefaultQuery("page", "1"); got != "1" {
		t.Fatalf("DefaultQuery = %q, want %q", got, "1")
	}
}

func TestDefaultQuery_Empty(t *testing.T) {
	r := httptest.NewRequest("GET", "/items?page=", nil)
	c := &Ctx{Request: r}

	if got := c.DefaultQuery("page", "1"); got != "1" {
		t.Fatalf("DefaultQuery = %q, want %q", got, "1")
	}
}

func TestParams(t *testing.T) {
	c := &Ctx{
		ps: params{
			{Key: "id", Value: "42"},
			{Key: "slug", Value: "hello-world"},
		},
	}

	m := c.Params()
	if len(m) != 2 {
		t.Fatalf("Params() len = %d, want 2", len(m))
	}
	if m["id"] != "42" {
		t.Fatalf("id = %q, want %q", m["id"], "42")
	}
	if m["slug"] != "hello-world" {
		t.Fatalf("slug = %q, want %q", m["slug"], "hello-world")
	}
}

func TestParams_Empty(t *testing.T) {
	c := &Ctx{}

	m := c.Params()
	if len(m) != 0 {
		t.Fatalf("Params() len = %d, want 0", len(m))
	}
}

func TestDefaultParam_Present(t *testing.T) {
	c := &Ctx{
		ps: params{{Key: "id", Value: "42"}},
	}

	if got := c.DefaultParam("id", "default"); got != "42" {
		t.Fatalf("DefaultParam = %q, want %q", got, "42")
	}
}

func TestDefaultParam_Missing(t *testing.T) {
	c := &Ctx{}

	if got := c.DefaultParam("id", "default"); got != "default" {
		t.Fatalf("DefaultParam = %q, want %q", got, "default")
	}
}

func TestDefaultParam_Empty(t *testing.T) {
	c := &Ctx{
		ps: params{{Key: "id", Value: ""}},
	}

	if got := c.DefaultParam("id", "default"); got != "default" {
		t.Fatalf("DefaultParam = %q, want %q", got, "default")
	}
}

func TestClientIP_XForwardedFor(t *testing.T) {
	r := httptest.NewRequest("GET", "/", nil)
	r.Header.Set("X-Forwarded-For", "1.2.3.4, 5.6.7.8")
	r.RemoteAddr = "127.0.0.1:12345"

	c := &Ctx{Request: r}
	if got := c.ClientIP(); got != "1.2.3.4" {
		t.Fatalf("ClientIP = %q, want %q", got, "1.2.3.4")
	}
}

func TestClientIP_XRealIP(t *testing.T) {
	r := httptest.NewRequest("GET", "/", nil)
	r.Header.Set("X-Real-IP", "10.0.0.1")
	r.RemoteAddr = "127.0.0.1:12345"

	c := &Ctx{Request: r}
	if got := c.ClientIP(); got != "10.0.0.1" {
		t.Fatalf("ClientIP = %q, want %q", got, "10.0.0.1")
	}
}

func TestClientIP_RemoteAddr(t *testing.T) {
	r := httptest.NewRequest("GET", "/", nil)
	r.RemoteAddr = "192.168.1.1:54321"

	c := &Ctx{Request: r}
	if got := c.ClientIP(); got != "192.168.1.1" {
		t.Fatalf("ClientIP = %q, want %q", got, "192.168.1.1")
	}
}

func TestClientIP_RemoteAddrNoPort(t *testing.T) {
	r := httptest.NewRequest("GET", "/", nil)
	r.RemoteAddr = "10.0.0.1"

	c := &Ctx{Request: r}
	if got := c.ClientIP(); got != "10.0.0.1" {
		t.Fatalf("ClientIP = %q, want %q", got, "10.0.0.1")
	}
}

func TestClientIP_NoHeaders(t *testing.T) {
	r := httptest.NewRequest("GET", "/", nil)
	r.RemoteAddr = "10.0.0.1:8080"

	c := &Ctx{Request: r}
	if got := c.ClientIP(); got != "10.0.0.1" {
		t.Fatalf("ClientIP = %q, want %q", got, "10.0.0.1")
	}
}

func TestHeaderXRequestID(t *testing.T) {
	if HeaderXRequestID != "X-Request-ID" {
		t.Fatalf("HeaderXRequestID = %q, want %q", HeaderXRequestID, "X-Request-ID")
	}
}

func TestQueryParam_Caching(t *testing.T) {
	r := httptest.NewRequest("GET", "/test?a=1&b=2&msg=hello%20world", nil)
	c := &Ctx{Request: r}

	if c.queryCache != nil {
		t.Fatal("queryCache should initially be nil")
	}

	a1 := c.QueryParam("a")
	if a1 != "1" {
		t.Fatalf("a = %q, want 1", a1)
	}

	// Single-key lookups scan the raw query without building the full map.
	if c.queryCache != nil {
		t.Fatal("queryCache should not be populated by single-key QueryParam lookups")
	}

	if got := c.QueryParam("msg"); got != "hello world" {
		t.Fatalf("msg = %q, want %q", got, "hello world")
	}

	// Reading all params builds and caches the map; later lookups use it.
	qp := c.QueryParams()
	if len(qp) != 3 {
		t.Fatalf("QueryParams() len = %d, want 3", len(qp))
	}
	if c.queryCache == nil {
		t.Fatal("queryCache should be populated after QueryParams")
	}

	b1 := c.QueryParam("b")
	if b1 != "2" {
		t.Fatalf("b = %q, want 2", b1)
	}

	// Reset should clear cache
	c.reset(nil, nil, nil)
	if c.queryCache != nil {
		t.Fatal("queryCache should be cleared after reset")
	}
}

func BenchmarkQueryParam_Cached(b *testing.B) {
	r := httptest.NewRequest("GET", "/test?a=1&b=2&c=3&d=4", nil)
	c := &Ctx{Request: r}

	b.ReportAllocs()
	for b.Loop() {
		_ = c.QueryParam("a")
		_ = c.QueryParam("b")
		_ = c.QueryParam("c")
		_ = c.QueryParam("d")
	}
}
