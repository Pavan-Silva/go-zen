package zen

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Pavan-Silva/go-zen/rbac"
)

type testAuthenticator struct {
	user *User
	err  error
}

func (a *testAuthenticator) Authenticate(_ *http.Request) (*User, error) {
	return a.user, a.err
}

var errUnauth = errors.New("unauthorized")

// seedAuthRoles installs the shared role graph used by permission middleware
// tests: viewer owns docs:read, editor owns docs:write and inherits viewer,
// admin owns admin:system.
func seedAuthRoles() {
	rbac.RegisterRoles(
		rbac.RoleConfig{Name: "viewer", Permissions: []string{"docs:read"}},
		rbac.RoleConfig{Name: "editor", Permissions: []string{"docs:write"}, InheritsFrom: []string{"viewer"}},
		rbac.RoleConfig{Name: "admin", Permissions: []string{"admin:system"}},
	)
}

func TestEnableAuth_Success(t *testing.T) {
	r := New(":0")
	r.Use(EnableAuth(&testAuthenticator{
		user: &User{ID: "1", Username: "john", Roles: []string{"admin"}},
	}))

	var captured *User
	r.GET("/protected", func(c *Ctx) {
		captured = c.GetUser()
		c.String(200, "ok")
	})

	req := httptest.NewRequest("GET", "/protected", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("status = %d, want 200; body: %s", w.Code, w.Body.String())
	}
	if captured == nil {
		t.Fatal("user not captured")
	}
	if captured.ID != "1" {
		t.Fatalf("id = %q, want %q", captured.ID, "1")
	}
}

func TestEnableAuth_Failure(t *testing.T) {
	r := New(":0")
	r.Use(EnableAuth(&testAuthenticator{err: errUnauth}))

	r.GET("/protected", func(c *Ctx) {
		c.String(200, "ok")
	})

	req := httptest.NewRequest("GET", "/protected", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != 401 {
		t.Fatalf("status = %d, want 401", w.Code)
	}
}

func TestEnableAuth_Skip(t *testing.T) {
	r := New(":0")
	r.Use(EnableAuth(&testAuthenticator{err: errUnauth}, SkipPaths("/public")))

	var captured bool
	r.GET("/public", func(c *Ctx) {
		captured = true
		c.String(200, "ok")
	})

	req := httptest.NewRequest("GET", "/public", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if !captured {
		t.Fatal("handler should be called for skipped path")
	}
	if w.Code != 200 {
		t.Fatalf("status = %d, want 200", w.Code)
	}
}

func TestRequireRole(t *testing.T) {
	r := New(":0")
	r.Use(EnableAuth(&testAuthenticator{
		user: &User{ID: "1", Username: "john", Roles: []string{"admin"}},
	}))
	r.Use(RequireRole("admin", nil))

	var captured bool
	r.GET("/admin", func(c *Ctx) {
		captured = true
		c.String(200, "ok")
	})

	req := httptest.NewRequest("GET", "/admin", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if !captured {
		t.Fatal("handler should be called for user with matching role")
	}
}

func TestRequireRole_Failure(t *testing.T) {
	r := New(":0")
	r.Use(EnableAuth(&testAuthenticator{
		user: &User{ID: "1", Username: "john", Roles: []string{"user"}},
	}))
	r.Use(RequireRole("admin", nil))

	r.GET("/admin", func(c *Ctx) {
		c.String(200, "ok")
	})

	req := httptest.NewRequest("GET", "/admin", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != 403 {
		t.Fatalf("status = %d, want 403", w.Code)
	}
}

func TestRequireClaim_AllowsMatchingClaim(t *testing.T) {
	r := New(":0")
	r.Use(EnableAuth(&testAuthenticator{
		user: &User{ID: "1", Username: "john", Claims: map[string]any{"tenant": "acme"}},
	}))
	r.GET("/documents", RequireClaim("tenant", "acme"), func(c *Ctx) {
		c.String(200, "ok")
	})

	req := httptest.NewRequest("GET", "/documents", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}
}

func TestRequireClaim_DeniesMismatchedClaim(t *testing.T) {
	r := New(":0")
	r.Use(EnableAuth(&testAuthenticator{
		user: &User{ID: "1", Username: "john", Claims: map[string]any{"tenant": "other"}},
	}))
	r.GET("/documents", RequireClaim("tenant", "acme"), func(c *Ctx) {
		c.String(200, "ok")
	})

	req := httptest.NewRequest("GET", "/documents", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403", w.Code)
	}
}

func TestGetUser_Nil(t *testing.T) {
	r := New(":0")
	r.GET("/no-auth", func(c *Ctx) {
		u := c.GetUser()
		if u != nil {
			t.Fatal("GetUser should return nil when not authenticated")
		}
		c.String(200, "ok")
	})

	req := httptest.NewRequest("GET", "/no-auth", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
}

func TestGetClaim(t *testing.T) {
	r := New(":0")
	r.Use(EnableAuth(&testAuthenticator{
		user: &User{ID: "1", Username: "john", Claims: map[string]any{"tenant": "acme"}},
	}))
	r.GET("/claim", func(c *Ctx) {
		value, ok := c.GetClaim("tenant")
		if !ok {
			t.Fatal("expected tenant claim to be found")
		}
		if value != "acme" {
			t.Fatalf("expected acme, got %v", value)
		}
		if _, ok := c.GetClaim("missing"); ok {
			t.Fatal("GetClaim should report a missing claim")
		}
		c.String(200, "ok")
	})

	req := httptest.NewRequest("GET", "/claim", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}
}

func TestGetClaim_NoUser(t *testing.T) {
	r := New(":0")
	r.GET("/no-auth", func(c *Ctx) {
		if _, ok := c.GetClaim("tenant"); ok {
			t.Fatal("GetClaim should be empty without an authenticated user")
		}
		c.String(200, "ok")
	})

	req := httptest.NewRequest("GET", "/no-auth", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}
}

func TestRequireAnyRoleMiddleware(t *testing.T) {
	r := New(":0")
	r.Use(EnableAuth(&testAuthenticator{
		user: &User{ID: "1", Username: "john", Roles: []string{"editor"}},
	}))
	r.GET("/docs", RequireAnyRole("admin", "editor"), func(c *Ctx) {
		c.String(200, "ok")
	})

	req := httptest.NewRequest("GET", "/docs", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}
}

func TestRequireAllRolesMiddleware(t *testing.T) {
	r := New(":0")
	r.Use(EnableAuth(&testAuthenticator{
		user: &User{ID: "1", Username: "john", Roles: []string{"admin", "editor"}},
	}))
	r.GET("/docs", RequireAllRoles("admin", "editor"), func(c *Ctx) {
		c.String(200, "ok")
	})

	req := httptest.NewRequest("GET", "/docs", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}
}

func TestRequireAllRolesMiddleware_Denied(t *testing.T) {
	r := New(":0")
	r.Use(EnableAuth(&testAuthenticator{
		user: &User{ID: "1", Username: "john", Roles: []string{"admin"}},
	}))
	r.GET("/docs", RequireAllRoles("admin", "editor"), func(c *Ctx) {
		c.String(200, "ok")
	})

	req := httptest.NewRequest("GET", "/docs", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403", w.Code)
	}
}

func TestRequireAnyPermission(t *testing.T) {
	seedAuthRoles()
	r := New(":0")
	r.Use(EnableAuth(&testAuthenticator{
		user: &User{ID: "1", Username: "john", Roles: []string{"viewer"}},
	}))
	r.GET("/docs", RequireAnyPermission("admin:system", "docs:read"), func(c *Ctx) {
		c.String(200, "ok")
	})

	req := httptest.NewRequest("GET", "/docs", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}
}

func TestRequireAllPermissions(t *testing.T) {
	seedAuthRoles()
	r := New(":0")
	r.Use(EnableAuth(&testAuthenticator{
		user: &User{ID: "1", Username: "john", Roles: []string{"editor"}},
	}))
	r.GET("/docs", RequireAllPermissions("docs:read", "docs:write"), func(c *Ctx) {
		c.String(200, "ok")
	})

	req := httptest.NewRequest("GET", "/docs", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}
}

func TestRequireAllPermissions_Denied(t *testing.T) {
	seedAuthRoles()
	r := New(":0")
	r.Use(EnableAuth(&testAuthenticator{
		user: &User{ID: "1", Username: "john", Roles: []string{"viewer"}},
	}))
	r.GET("/docs", RequireAllPermissions("docs:read", "docs:write"), func(c *Ctx) {
		c.String(200, "ok")
	})

	req := httptest.NewRequest("GET", "/docs", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403", w.Code)
	}
}

func TestSkipPaths(t *testing.T) {
	skip := SkipPaths("/health", "/ready")

	tests := []struct {
		path string
		want bool
	}{
		{"/health", true},
		{"/ready", true},
		{"/users", false},
		{"/health/check", false},
	}

	for _, tt := range tests {
		req := httptest.NewRequest("GET", tt.path, nil)
		got := skip(req)
		if got != tt.want {
			t.Errorf("SkipPaths(%q) = %v, want %v", tt.path, got, tt.want)
		}
	}
}

func TestSkipPrefixes(t *testing.T) {
	skip := SkipPrefixes("/public", "/api/v1")

	tests := []struct {
		path string
		want bool
	}{
		{"/public", true},
		{"/public/css/style.css", true},
		{"/api/v1/users", true},
		{"/users", false},
		{"/api/v2/users", false},
		{"/", false},
	}

	for _, tt := range tests {
		req := httptest.NewRequest("GET", tt.path, nil)
		got := skip(req)
		if got != tt.want {
			t.Errorf("SkipPrefixes(%q) = %v, want %v", tt.path, got, tt.want)
		}
	}
}

func TestSkipMethodsAndPaths(t *testing.T) {
	skip := SkipMethodsAndPaths("GET", "/health", "/ready")

	tests := []struct {
		method string
		path   string
		want   bool
	}{
		{"GET", "/health", true},
		{"GET", "/ready", true},
		{"POST", "/health", false},
		{"GET", "/users", false},
	}

	for _, tt := range tests {
		req := httptest.NewRequest(tt.method, tt.path, nil)
		got := skip(req)
		if got != tt.want {
			t.Errorf("SkipMethodsAndPaths(%s %s) = %v, want %v", tt.method, tt.path, got, tt.want)
		}
	}
}

func TestRequirePermission_Allowed(t *testing.T) {
	seedAuthRoles()
	r := New(":0")
	r.Use(EnableAuth(&testAuthenticator{
		user: &User{
			ID: "1", Username: "john",
			Roles: []string{"editor"},
		},
	}))
	r.GET("/docs", RequirePermission("docs:write"), func(c *Ctx) {
		c.String(200, "ok")
	})

	req := httptest.NewRequest("GET", "/docs", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("status = %d, want 200; body: %s", w.Code, w.Body.String())
	}
}

func TestRequirePermission_Denied(t *testing.T) {
	seedAuthRoles()
	r := New(":0")
	r.Use(EnableAuth(&testAuthenticator{
		user: &User{
			ID: "1", Username: "john",
			Roles: []string{"viewer"},
		},
	}))
	r.GET("/admin", RequirePermission("admin:system"), func(c *Ctx) {
		c.String(200, "ok")
	})

	req := httptest.NewRequest("GET", "/admin", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != 403 {
		t.Fatalf("status = %d, want 403; body: %s", w.Code, w.Body.String())
	}
}

func TestRequirePermission_NoUser(t *testing.T) {
	r := New(":0")
	r.GET("/noauth", RequirePermission("read:anything"), func(c *Ctx) {
		c.String(200, "ok")
	})

	req := httptest.NewRequest("GET", "/noauth", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != 403 {
		t.Fatalf("status = %d, want 403; body: %s", w.Code, w.Body.String())
	}
}

func TestEngine_EnableAuth(t *testing.T) {
	r := New(":0")
	r.EnableAuth(&testAuthenticator{
		user: &User{ID: "1", Username: "john", Roles: []string{"admin"}},
	})

	var captured *User
	r.GET("/protected", func(c *Ctx) {
		captured = c.GetUser()
		c.String(200, "ok")
	})

	req := httptest.NewRequest("GET", "/protected", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("status = %d, want 200", w.Code)
	}
	if captured == nil || captured.ID != "1" {
		t.Fatalf("Engine.EnableAuth did not authenticate user, captured=%v", captured)
	}
}

func TestUser_HasRole(t *testing.T) {
	u := User{Roles: []string{"admin", "user"}}

	if !u.HasRole("admin") {
		t.Fatal("should have admin role")
	}
	if u.HasRole("superadmin") {
		t.Fatal("should not have superadmin role")
	}
}

func TestUser_HasRole_ExactMatch(t *testing.T) {
	u := User{Roles: []string{"admin"}}

	if u.HasRole("Admin") {
		t.Fatal("role names must match exactly, no case folding")
	}
	if u.HasRole("ROLE_admin") {
		t.Fatal("role names must match exactly, no prefix normalization")
	}
}

func TestUser_HasAnyRole(t *testing.T) {
	u := User{Roles: []string{"editor"}}

	if !u.HasAnyRole("admin", "editor") {
		t.Fatal("should match any listed role")
	}
	if u.HasAnyRole("admin", "user") {
		t.Fatal("should not match unrelated roles")
	}
	var nilUser *User
	if nilUser.HasAnyRole("admin") {
		t.Fatal("nil user must not have roles")
	}
}

func TestUser_HasAllRoles(t *testing.T) {
	u := User{Roles: []string{"admin", "editor"}}

	if !u.HasAllRoles("admin", "editor") {
		t.Fatal("should match all listed roles")
	}
	if u.HasAllRoles("admin", "publisher") {
		t.Fatal("should require every role")
	}
	if u.HasAllRoles() {
		t.Fatal("empty role list must not grant access")
	}
}

func TestUser_HasPermission(t *testing.T) {
	seedAuthRoles()
	u := User{Roles: []string{"editor"}}

	if !u.HasPermission("docs:write") {
		t.Fatal("should have own permission")
	}
	if !u.HasPermission("docs:read") {
		t.Fatal("should have inherited permission")
	}
	if u.HasPermission("admin:system") {
		t.Fatal("should not have unrelated permission")
	}
}

func TestUser_HasPermission_UnregisteredRole(t *testing.T) {
	seedAuthRoles()
	u := User{Roles: []string{"ghost"}}

	if u.HasPermission("docs:read") {
		t.Fatal("unregistered roles must grant nothing")
	}
}

func TestUser_HasAnyPermission(t *testing.T) {
	seedAuthRoles()
	u := User{Roles: []string{"viewer"}}

	if !u.HasAnyPermission("admin:system", "docs:read") {
		t.Fatal("should match any listed permission")
	}
	if u.HasAnyPermission("admin:system", "orders:write") {
		t.Fatal("should not match unrelated permissions")
	}
}

func TestUser_HasAllPermissions(t *testing.T) {
	seedAuthRoles()
	u := User{Roles: []string{"editor"}}

	if !u.HasAllPermissions("docs:read", "docs:write") {
		t.Fatal("should match all listed permissions")
	}
	if u.HasAllPermissions("docs:read", "admin:system") {
		t.Fatal("should require all permissions")
	}
	if u.HasAllPermissions() {
		t.Fatal("empty permission list must not grant access")
	}
}

func TestHasPermission_Exists(t *testing.T) {
	seedAuthRoles()
	u := User{Roles: []string{"viewer"}}
	if !u.HasPermission("docs:read") {
		t.Fatal("expected HasPermission to return true")
	}
}

func TestHasPermission_NotExists(t *testing.T) {
	seedAuthRoles()
	u := User{Roles: []string{"viewer"}}
	if u.HasPermission("admin:system") {
		t.Fatal("expected HasPermission to return false")
	}
}

func TestHasPermission_NilUser(t *testing.T) {
	var u User
	if u.HasPermission("anything") {
		t.Fatal("expected HasPermission to return false for nil user")
	}
}

// Regression: an empty permission string must never grant access, even when
// the user holds a role.
func TestHasPermission_EmptyNeverGrants(t *testing.T) {
	seedAuthRoles()
	u := &User{Roles: []string{"viewer"}}
	if u.HasPermission("") {
		t.Error("empty permission must not match")
	}
	if (&User{}).HasPermission("") {
		t.Error("empty permission must not match")
	}
	var nilUser *User
	if nilUser.HasPermission("docs:read") {
		t.Error("nil user must not have permissions")
	}
}
