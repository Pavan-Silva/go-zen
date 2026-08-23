package openapi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Pavan-Silva/go-zen"
	"github.com/Pavan-Silva/go-zen/internal/system"
	"github.com/swaggo/swag"
)

// fakeSwag implements swag.Swagger with a canned document. A unique instance
// name is required per test because swag.Register panics on duplicates.
type fakeSwag struct {
	doc string
}

func (f fakeSwag) ReadDoc() string { return f.doc }

func registerFakeSwag(t *testing.T, doc string) string {
	t.Helper()
	name := "test-" + t.Name()
	swag.Register(name, fakeSwag{doc: doc})
	return name
}

const sampleSpec = `{
  "swagger": "2.0",
  "info": {"title": "Test API", "version": "1.0.0"},
  "paths": {"/users/{id}": {"get": {"summary": "Get user"}}}
}`

func TestNew(t *testing.T) {
	doc := New(Config{})
	if doc.cfg.SpecPath != "/openapi.json" {
		t.Fatalf("expected default spec path /openapi.json, got %s", doc.cfg.SpecPath)
	}
	if doc.cfg.DocPath != "/docs" {
		t.Fatalf("expected default doc path /docs, got %s", doc.cfg.DocPath)
	}
	if doc.cfg.SwagInstance != swag.Name {
		t.Fatalf("expected default swag instance %q, got %q", swag.Name, doc.cfg.SwagInstance)
	}
}

func TestConfigCustomPaths(t *testing.T) {
	doc := New(Config{SpecPath: "/api/openapi.json", DocPath: "/api/docs"})
	if doc.cfg.SpecPath != "/api/openapi.json" {
		t.Fatalf("expected /api/openapi.json, got %s", doc.cfg.SpecPath)
	}
	if doc.cfg.DocPath != "/api/docs" {
		t.Fatalf("expected /api/docs, got %s", doc.cfg.DocPath)
	}
}

func TestSpecJSON(t *testing.T) {
	doc := New(Config{SwagInstance: registerFakeSwag(t, sampleSpec)})

	data, err := doc.SpecJSON()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var spec map[string]any
	if err := json.Unmarshal(data, &spec); err != nil {
		t.Fatalf("failed to unmarshal spec: %v", err)
	}
	if spec["swagger"] != "2.0" {
		t.Fatalf("expected swagger 2.0 document, got %v", spec["swagger"])
	}

	// Cached: second call returns identical bytes without re-reading.
	again, _ := doc.SpecJSON()
	if &data[0] != &again[0] {
		t.Fatal("expected cached spec bytes")
	}
}

func TestSpecJSONMissingRegistration(t *testing.T) {
	doc := New(Config{SwagInstance: "does-not-exist-" + t.Name()})
	if _, err := doc.SpecJSON(); err == nil {
		t.Fatal("expected error when no swag documentation is registered")
	}
}

func TestWriteSpecUnavailable(t *testing.T) {
	doc := New(Config{SwagInstance: "missing-" + t.Name()})
	handler := doc.SpecHandler()

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/openapi.json", nil)
	c := &zen.Ctx{Response: w, Request: r}
	handler(c)

	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503 when spec unavailable, got %d", w.Code)
	}
	if ct := w.Header().Get("Content-Type"); ct != "application/json" {
		t.Fatalf("expected application/json error body, got %s", ct)
	}
}

func TestWriteSpec(t *testing.T) {
	doc := New(Config{SwagInstance: registerFakeSwag(t, sampleSpec)})
	handler := doc.SpecHandler()

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/openapi.json", nil)
	c := &zen.Ctx{Response: w, Request: r}
	handler(c)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	if ct := w.Header().Get("Content-Type"); ct != "application/json" {
		t.Fatalf("expected application/json, got %s", ct)
	}

	var spec map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &spec); err != nil {
		t.Fatalf("invalid json: %v", err)
	}
}

func TestDocHandler(t *testing.T) {
	doc := New(Config{})
	handler := doc.DocHandler()

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/docs", nil)
	c := &zen.Ctx{Response: w, Request: r}
	handler(c)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	if ct := w.Header().Get("Content-Type"); ct != "text/html; charset=utf-8" {
		t.Fatalf("expected text/html, got %s", ct)
	}
	body := w.Body.String()
	if len(body) == 0 {
		t.Fatal("expected non-empty body")
	}
}

func TestDocHandlerScalarUI(t *testing.T) {
	doc := New(Config{})
	body := uiHTML(doc)
	if !strings.Contains(body, "createApiReference") {
		t.Fatal("expected Scalar createApiReference bootstrap in HTML")
	}
	if !strings.Contains(body, `url: "/openapi.json"`) {
		t.Fatal("expected default spec URL in HTML")
	}
}

func TestDocHandlerFallbackWhenAssetsMissing(t *testing.T) {
	prevTemplate := uiTemplate
	prevAssets := uiAssets
	uiTemplate = ""
	uiAssets = nil
	t.Cleanup(func() {
		uiTemplate = prevTemplate
		uiAssets = prevAssets
	})

	doc := New(Config{})
	handler := doc.DocHandler()

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/docs", nil)
	c := &zen.Ctx{Response: w, Request: r}
	handler(c)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	if body := w.Body.String(); !strings.Contains(body, "fallback mode") {
		t.Fatalf("expected fallback HTML content, got %s", body)
	}
}

// Literal '%' characters in the embedded UI HTML must survive substitution
// untouched (fmt.Sprintf would corrupt them).
func TestUIHTMLLiteralPercent(t *testing.T) {
	prevTemplate := uiTemplate
	uiTemplate = `<html><div style="width:100%">url="%[1]s" v="%[2]s"</div></html>`
	t.Cleanup(func() { uiTemplate = prevTemplate })

	doc := New(Config{SpecPath: "/spec.json"})
	html := uiHTML(doc)

	if strings.Contains(html, "%!") {
		t.Fatalf("HTML corrupted by format-verb interpretation: %s", html)
	}
	if !strings.Contains(html, `url="/spec.json"`) || !strings.Contains(html, `v="`+system.Version+`"`) {
		t.Fatalf("placeholders not substituted: %s", html)
	}
	if !strings.Contains(html, "width:100%") {
		t.Fatalf("literal %% lost: %s", html)
	}
}

func TestDocDisabled(t *testing.T) {
	doc := New(Config{SpecPath: "/spec.json", DisableUI: true, SwagInstance: registerFakeSwag(t, sampleSpec)})
	r := zen.New(":0")
	doc.RegisterRoutes(r)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/spec.json", nil)
	r.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 from spec, got %d", rec.Code)
	}

	rec2 := httptest.NewRecorder()
	req2 := httptest.NewRequest(http.MethodGet, "/docs", nil)
	r.ServeHTTP(rec2, req2)
	if rec2.Code != http.StatusNotFound {
		t.Fatalf("expected 404 when UI disabled, got %d", rec2.Code)
	}
}

func TestRegisterRoutes(t *testing.T) {
	doc := New(Config{SwagInstance: registerFakeSwag(t, sampleSpec)})

	r := zen.New(":0")
	doc.RegisterRoutes(r)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/openapi.json", nil)
	r.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 from spec endpoint, got %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), `"swagger"`) {
		t.Fatalf("expected generated spec body, got %s", rec.Body.String())
	}

	rec2 := httptest.NewRecorder()
	req2 := httptest.NewRequest(http.MethodGet, "/docs", nil)
	r.ServeHTTP(rec2, req2)
	if rec2.Code != http.StatusOK {
		t.Fatalf("expected 200 from doc endpoint, got %d", rec2.Code)
	}
}

func TestCustomInstanceName(t *testing.T) {
	name := registerFakeSwag(t, sampleSpec)
	doc := New(Config{SwagInstance: name})

	data, err := doc.SpecJSON()
	if err != nil {
		t.Fatalf("unexpected error for custom instance %q: %v", name, err)
	}
	if !json.Valid(data) {
		t.Fatal("expected valid JSON from custom swag instance")
	}
}
