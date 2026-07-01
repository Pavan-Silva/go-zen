package openapi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Pavan-Silva/go-zen"
)

type User struct {
	ID    int    `json:"id" validate:"required"`
	Name  string `json:"name" validate:"required"`
	Email string `json:"email,omitempty"`
}

type CreateUserRequest struct {
	Name  string `json:"name" validate:"required"`
	Email string `json:"email" validate:"required,email"`
}

type ErrorResponse struct {
	Error string `json:"error"`
}

type UserResponse struct {
	ID    int    `json:"id"`
	Name  string `json:"name"`
	Email string `json:"email"`
	Role  string `json:"role"`
}

type ListUsersResponse struct {
	Users []UserResponse `json:"users"`
	Total int            `json:"total"`
}

func TestNew(t *testing.T) {
	doc := New(Config{Title: "Test API", Version: "1.0.0"})
	if doc.cfg.Title != "Test API" {
		t.Fatalf("expected title Test API, got %s", doc.cfg.Title)
	}
	if doc.cfg.SpecPath != "/openapi.json" {
		t.Fatalf("expected default spec path /openapi.json, got %s", doc.cfg.SpecPath)
	}
	if doc.cfg.DocPath != "/docs" {
		t.Fatalf("expected default doc path /docs, got %s", doc.cfg.DocPath)
	}
}

func TestConfigCustomPaths(t *testing.T) {
	doc := New(Config{
		Title:    "Test",
		Version:  "1.0",
		SpecPath: "/api/openapi.json",
		DocPath:  "/api/docs",
	})
	if doc.cfg.SpecPath != "/api/openapi.json" {
		t.Fatalf("expected /api/openapi.json, got %s", doc.cfg.SpecPath)
	}
	if doc.cfg.DocPath != "/api/docs" {
		t.Fatalf("expected /api/docs, got %s", doc.cfg.DocPath)
	}
}

func TestRegisterEnrichment(t *testing.T) {
	doc := New(Config{Title: "Test", Version: "1.0"})
	doc.Register("GET", "/users/{id}", RI().
		Summary("Get user by ID").
		Desc("Returns a single user").
		Resp(200, &User{}).
		Resp(404, &ErrorResponse{}),
	)

	doc.mu.RLock()
	info := doc.routes["GET"]["/users/{id}"]
	doc.mu.RUnlock()

	if info.Summary != "Get user by ID" {
		t.Fatalf("expected summary 'Get user by ID', got %s", info.Summary)
	}
}

func TestSpecJSON(t *testing.T) {
	doc := New(Config{Title: "Test API", Version: "1.0.0", Description: "Test"})
	doc.Register("GET", "/users/{id}", RI().
		Summary("Get user").
		Resp(200, &User{}),
	)
	doc.Register("POST", "/users", RI().
		Summary("Create user").
		Body(&CreateUserRequest{}).
		Resp(201, &User{}),
	)

	data := doc.SpecJSON()
	var spec map[string]any
	if err := json.Unmarshal(data, &spec); err != nil {
		t.Fatalf("failed to unmarshal spec: %v", err)
	}

	info := spec["info"].(map[string]any)
	if info["title"] != "Test API" {
		t.Fatalf("expected title 'Test API', got %v", info["title"])
	}

	paths := spec["paths"].(map[string]any)
	if _, ok := paths["/users/{id}"]; !ok {
		t.Fatal("expected path /users/{id}")
	}
	if _, ok := paths["/users"]; !ok {
		t.Fatal("expected path /users")
	}

	components := spec["components"].(map[string]any)
	schemas := components["schemas"].(map[string]any)
	if _, ok := schemas["User"]; !ok {
		t.Fatal("expected schema User")
	}
	if _, ok := schemas["CreateUserRequest"]; !ok {
		t.Fatal("expected schema CreateUserRequest")
	}
}

func TestSpecHandler(t *testing.T) {
	doc := New(Config{Title: "Test", Version: "1.0"})
	doc.Register("GET", "/ping", RI().Summary("Ping"))

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
	if spec["openapi"] != "3.0.3" {
		t.Fatalf("expected openapi 3.0.3, got %v", spec["openapi"])
	}
}

func TestDocHandler(t *testing.T) {
	doc := New(Config{Title: "Test", Version: "1.0"})
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

func TestDocHandlerSwaggerUI(t *testing.T) {
	doc := New(Config{Title: "Test", Version: "1.0"})
	body := uiHTML(doc)
	if !strings.Contains(body, "swagger-ui") {
		t.Fatal("expected swagger-ui in HTML")
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

	doc := New(Config{Title: "Test", Version: "1.0"})
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

func TestDocDisabled(t *testing.T) {
	doc := New(Config{Title: "Test", Version: "1.0", SpecPath: "/spec.json", DisableUI: true})
	r := zen.New(":0")
	doc.RegisterRoutes(r)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/spec.json", nil)
	r.Mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 from spec, got %d", rec.Code)
	}

	rec2 := httptest.NewRecorder()
	req2 := httptest.NewRequest(http.MethodGet, "/docs", nil)
	r.Mux.ServeHTTP(rec2, req2)
	if rec2.Code != http.StatusNotFound {
		t.Fatalf("expected 404 when UI disabled, got %d", rec2.Code)
	}
}

func TestRegisterRoutes(t *testing.T) {
	doc := New(Config{Title: "Test", Version: "1.0"})

	r := zen.New(":0")
	doc.RegisterRoutes(r)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/openapi.json", nil)
	r.Mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 from spec endpoint, got %d", rec.Code)
	}

	rec2 := httptest.NewRecorder()
	req2 := httptest.NewRequest(http.MethodGet, "/docs", nil)
	r.Mux.ServeHTTP(rec2, req2)
	if rec2.Code != http.StatusOK {
		t.Fatalf("expected 200 from doc endpoint, got %d", rec2.Code)
	}
}

func TestSpecIdempotent(t *testing.T) {
	doc := New(Config{Title: "Test", Version: "1.0"})
	doc.Register("GET", "/a", RI().Summary("A"))
	first := doc.SpecJSON()
	doc.Register("GET", "/b", RI().Summary("B"))
	second := doc.SpecJSON()
	if len(first) >= len(second) {
		t.Fatal("expected spec to grow after adding route")
	}
}

func TestRequiredFieldsInSchema(t *testing.T) {
	doc := New(Config{Title: "Test", Version: "1.0"})
	doc.Register("POST", "/users", RI().
		Body(&CreateUserRequest{}).
		Resp(201, &User{}),
	)

	data := doc.SpecJSON()
	var spec map[string]any
	json.Unmarshal(data, &spec)
	components := spec["components"].(map[string]any)
	schemas := components["schemas"].(map[string]any)

	userSchema := schemas["User"].(map[string]any)
	req := userSchema["required"].([]any)
	if len(req) != 2 {
		t.Fatalf("expected 2 required fields for User, got %d: %v", len(req), req)
	}
}

func TestEngineIntegration(t *testing.T) {
	doc := New(Config{Title: "Integration", Version: "1.0"})
	r := zen.New(":0")

	r.GET("/users/{id}", func(c *zen.Ctx) {
		c.JSON(200, map[string]string{"id": c.Param("id")})
	})

	doc.Register("GET", "/users/{id}", RI().
		Summary("Get user").
		Resp(200, &User{}),
	)

	doc.RegisterRoutes(r)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/openapi.json", nil)
	r.Mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	var spec map[string]any
	json.Unmarshal(rec.Body.Bytes(), &spec)
	paths := spec["paths"].(map[string]any)
	if _, ok := paths["/users/{id}"]; !ok {
		t.Fatal("expected /users/{id} in paths")
	}
}

func TestEmptySpec(t *testing.T) {
	doc := New(Config{Title: "Empty", Version: "0.1"})
	data := doc.SpecJSON()
	var spec map[string]any
	json.Unmarshal(data, &spec)
	paths := spec["paths"].(map[string]any)
	if len(paths) != 0 {
		t.Fatal("expected empty paths")
	}
}

func TestRIFluidAPI(t *testing.T) {
	b := RI().
		Summary("Get user").
		Desc("Returns a user by ID").
		Tags("Users", "Admin").
		Resp(200, &UserResponse{}).
		Resp(404, &ErrorResponse{}).
		Body(&CreateUserRequest{})

	if b.info.Summary != "Get user" {
		t.Fatalf("expected summary 'Get user', got %q", b.info.Summary)
	}
	if b.info.Description != "Returns a user by ID" {
		t.Fatalf("expected description 'Returns a user by ID', got %q", b.info.Description)
	}
	if len(b.info.Tags) != 2 || b.info.Tags[0] != "Users" || b.info.Tags[1] != "Admin" {
		t.Fatalf("expected tags [Users Admin], got %v", b.info.Tags)
	}
	if b.info.RequestBody == nil {
		t.Fatal("expected request body")
	}
	if len(b.info.Responses) != 2 {
		t.Fatalf("expected 2 responses, got %d", len(b.info.Responses))
	}
	if _, ok := b.info.Responses[200]; !ok {
		t.Fatal("expected 200 response")
	}
}

func TestRIDeprecated(t *testing.T) {
	b := RI().Summary("Old").Deprecated()
	if !b.info.Deprecated {
		t.Fatal("expected deprecated")
	}
}

func TestMethodHelpers(t *testing.T) {
	doc := New(Config{Title: "Test", Version: "1.0"})

	doc.GET("/items", RI().Summary("List").Resp(200, &ListUsersResponse{}))
	doc.POST("/items", RI().Summary("Create").Body(&CreateUserRequest{}).Resp(201, &UserResponse{}))
	doc.DELETE("/items/{id}", RI().Summary("Delete").Resp(204, nil))

	data := doc.SpecJSON()
	var spec map[string]any
	json.Unmarshal(data, &spec)
	paths := spec["paths"].(map[string]any)
	items := paths["/items"].(map[string]any)
	if _, ok := items["get"]; !ok {
		t.Fatal("expected GET /items")
	}
	if _, ok := items["post"]; !ok {
		t.Fatal("expected POST /items")
	}
	itemsID := paths["/items/{id}"].(map[string]any)
	if _, ok := itemsID["delete"]; !ok {
		t.Fatal("expected DELETE /items/{id}")
	}
}

func TestRIEmpty(t *testing.T) {
	b := RI()
	if b.info.Summary != "" || b.info.Description != "" || b.info.Tags != nil || b.info.Responses != nil {
		t.Fatal("expected empty RI")
	}
}

type empty struct{}
type nested struct {
	Inner struct{ X string }
	Value int
}

func TestNestedStructSchema(t *testing.T) {
	doc := New(Config{Title: "Test", Version: "1.0"})
	doc.Register("GET", "/nested", RI().Resp(200, &nested{}))

	data := doc.SpecJSON()
	var spec map[string]any
	json.Unmarshal(data, &spec)
	components := spec["components"].(map[string]any)
	schemas := components["schemas"].(map[string]any)

	if _, ok := schemas["nested"]; !ok {
		t.Fatal("expected schema for nested")
	}
	s := schemas["nested"].(map[string]any)
	props := s["properties"].(map[string]any)
	if _, ok := props["Inner"]; !ok {
		t.Fatal("expected Inner property")
	}
}

func TestNilModelResponse(t *testing.T) {
	doc := New(Config{Title: "Test", Version: "1.0"})
	doc.Register("DELETE", "/users/{id}", RI().
		Summary("Delete user").
		Resp(204, nil).
		Resp(404, &ErrorResponse{}),
	)
	data := doc.SpecJSON()
	var spec map[string]any
	json.Unmarshal(data, &spec)
	paths := spec["paths"].(map[string]any)
	deleteOp := paths["/users/{id}"].(map[string]any)["delete"].(map[string]any)
	resp := deleteOp["responses"].(map[string]any)
	if _, ok := resp["204"]; !ok {
		t.Fatal("expected 204 response")
	}
	if _, ok := resp["404"]; !ok {
		t.Fatal("expected 404 response")
	}
}

func TestContextPoolIntegration(t *testing.T) {
	doc := New(Config{Title: "Pool", Version: "1.0"})
	doc.Register("GET", "/pool", RI().Summary("Pool test"))

	handler := doc.SpecHandler()
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/openapi.json", nil)
	c := &zen.Ctx{Response: w, Request: r}
	handler(c)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}
