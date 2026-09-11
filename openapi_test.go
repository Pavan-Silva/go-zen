package zen

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

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

const openapiSampleSpec = `{
  "swagger": "2.0",
  "info": {"title": "Test API", "version": "1.0.0"},
  "paths": {"/users/{id}": {"get": {"summary": "Get user"}}}
}`

func TestNewOpenAPI(t *testing.T) {
	doc := NewOpenAPI(OpenAPIConfig{})
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

func TestOpenAPIConfigCustomPaths(t *testing.T) {
	doc := NewOpenAPI(OpenAPIConfig{SpecPath: "/api/openapi.json", DocPath: "/api/docs"})
	if doc.cfg.SpecPath != "/api/openapi.json" {
		t.Fatalf("expected /api/openapi.json, got %s", doc.cfg.SpecPath)
	}
	if doc.cfg.DocPath != "/api/docs" {
		t.Fatalf("expected /api/docs, got %s", doc.cfg.DocPath)
	}
}

func TestOpenAPISpecJSON(t *testing.T) {
	doc := NewOpenAPI(OpenAPIConfig{SwagInstance: registerFakeSwag(t, openapiSampleSpec)})

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

func TestOpenAPISpecJSONMissingRegistration(t *testing.T) {
	doc := NewOpenAPI(OpenAPIConfig{SwagInstance: "does-not-exist-" + t.Name()})
	if _, err := doc.SpecJSON(); err == nil {
		t.Fatal("expected error when no swag documentation is registered")
	}
}

func TestOpenAPIWriteSpecUnavailable(t *testing.T) {
	doc := NewOpenAPI(OpenAPIConfig{SwagInstance: "missing-" + t.Name()})
	handler := doc.SpecHandler()

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/openapi.json", nil)
	handler.ServeHTTP(w, r)

	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503 when spec unavailable, got %d", w.Code)
	}
	if ct := w.Header().Get("Content-Type"); ct != "application/json" {
		t.Fatalf("expected application/json error body, got %s", ct)
	}
}

func TestOpenAPIWriteSpec(t *testing.T) {
	doc := NewOpenAPI(OpenAPIConfig{SwagInstance: registerFakeSwag(t, openapiSampleSpec)})
	handler := doc.SpecHandler()

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/openapi.json", nil)
	handler.ServeHTTP(w, r)

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

func TestOpenAPIDocHandler(t *testing.T) {
	doc := NewOpenAPI(OpenAPIConfig{})
	handler := doc.DocHandler()

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/docs", nil)
	handler.ServeHTTP(w, r)

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

func TestOpenAPIDocDisabled(t *testing.T) {
	doc := NewOpenAPI(OpenAPIConfig{SpecPath: "/spec.json", DisableUI: true, SwagInstance: registerFakeSwag(t, openapiSampleSpec)})
	e := New(":0")
	doc.RegisterRoutes(&e.RouterGroup)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/spec.json", nil)
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 from spec, got %d", rec.Code)
	}

	rec2 := httptest.NewRecorder()
	req2 := httptest.NewRequest(http.MethodGet, "/docs", nil)
	e.ServeHTTP(rec2, req2)
	if rec2.Code != http.StatusNotFound {
		t.Fatalf("expected 404 when UI disabled, got %d", rec2.Code)
	}
}

func TestOpenAPIRegisterRoutes(t *testing.T) {
	doc := NewOpenAPI(OpenAPIConfig{SwagInstance: registerFakeSwag(t, openapiSampleSpec)})

	e := New(":0")
	doc.RegisterRoutes(&e.RouterGroup)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/openapi.json", nil)
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 from spec endpoint, got %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), `"swagger"`) {
		t.Fatalf("expected generated spec body, got %s", rec.Body.String())
	}

	rec2 := httptest.NewRecorder()
	req2 := httptest.NewRequest(http.MethodGet, "/docs", nil)
	e.ServeHTTP(rec2, req2)
	if rec2.Code != http.StatusOK {
		t.Fatalf("expected 200 from doc endpoint, got %d", rec2.Code)
	}
}

func TestOpenAPICustomInstanceName(t *testing.T) {
	name := registerFakeSwag(t, openapiSampleSpec)
	doc := NewOpenAPI(OpenAPIConfig{SwagInstance: name})

	data, err := doc.SpecJSON()
	if err != nil {
		t.Fatalf("unexpected error for custom instance %q: %v", name, err)
	}
	if !json.Valid(data) {
		t.Fatal("expected valid JSON from custom swag instance")
	}
}

func TestEngineEnableAPIDocs(t *testing.T) {
	r := New(":0")
	r.EnableAPIDocs()

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/openapi.json", nil)
	r.ServeHTTP(rec, req)
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503 without registered docs, got %d", rec.Code)
	}

	rec2 := httptest.NewRecorder()
	req2 := httptest.NewRequest(http.MethodGet, "/docs", nil)
	r.ServeHTTP(rec2, req2)
	if rec2.Code != http.StatusOK {
		t.Fatalf("expected 200 from doc endpoint, got %d", rec2.Code)
	}
}

func TestEngineEnableAPIDocsConfigOptions(t *testing.T) {
	r := New(":0")
	swag.Register("cfg-"+t.Name(), fakeSwag{doc: openapiSampleSpec})
	r.EnableAPIDocs(
		OpenAPIConfig{SpecPath: "/spec.json", DocPath: "/api/docs", SwagInstance: "cfg-" + t.Name()},
		func(c *OpenAPIConfig) { c.DisableUI = true },
	)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/spec.json", nil)
	r.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 from custom spec path, got %d (body %s)", rec.Code, rec.Body.String())
	}

	rec2 := httptest.NewRecorder()
	req2 := httptest.NewRequest(http.MethodGet, "/api/docs", nil)
	r.ServeHTTP(rec2, req2)
	if rec2.Code != http.StatusNotFound {
		t.Fatalf("expected 404 when UI disabled, got %d", rec2.Code)
	}
}

func TestEngineEnableAPIDocsPanicsOnBadArg(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("expected panic for unsupported argument type")
		}
	}()
	New(":0").EnableAPIDocs("nope")
}
