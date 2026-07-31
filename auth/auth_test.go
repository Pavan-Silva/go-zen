package auth

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Pavan-Silva/go-zen"
	"github.com/golang-jwt/jwt/v5"
)

type testAuth struct {
	user *User
	err  error
}

func (a *testAuth) Authenticate(r *http.Request) (*User, error) {
	return a.user, a.err
}

func TestRequireAuth_Success(t *testing.T) {
	r := zen.New(":0")
	r.Use(RequireAuth(&testAuth{
		user: &User{ID: "1", Username: "john", Authorities: []string{"ROLE_ADMIN"}},
	}))

	var captured *User
	r.GET("/protected", func(c *zen.Ctx) {
		captured = GetUser(c)
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

func TestRequireAuth_Failure(t *testing.T) {
	r := zen.New(":0")
	r.Use(RequireAuth(&testAuth{err: errUnauth}))

	r.GET("/protected", func(c *zen.Ctx) {
		c.String(200, "ok")
	})

	req := httptest.NewRequest("GET", "/protected", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != 401 {
		t.Fatalf("status = %d, want 401", w.Code)
	}
}

var errUnauth = &testError{"unauthorized"}

type testError struct {
	msg string
}

func (e *testError) Error() string {
	return e.msg
}

func TestRequireAuth_Skip(t *testing.T) {
	r := zen.New(":0")
	r.Use(RequireAuth(&testAuth{err: errUnauth}, SkipPaths("/public")))

	var captured bool
	r.GET("/public", func(c *zen.Ctx) {
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
	r := zen.New(":0")
	r.Use(RequireAuth(&testAuth{
		user: &User{ID: "1", Username: "john", Authorities: []string{"role:admin"}},
	}))
	r.Use(RequireRole("admin", nil))

	var captured bool
	r.GET("/admin", func(c *zen.Ctx) {
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
	r := zen.New(":0")
	r.Use(RequireAuth(&testAuth{
		user: &User{ID: "1", Username: "john", Authorities: []string{"role:user"}},
	}))
	r.Use(RequireRole("admin", nil))

	r.GET("/admin", func(c *zen.Ctx) {
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
	r := zen.New(":0")
	r.Use(RequireAuth(&testAuth{
		user: &User{ID: "1", Username: "john", Claims: map[string]any{"tenant": "acme"}},
	}))
	r.GET("/documents", RequireClaim("tenant", "acme"), func(c *zen.Ctx) {
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
	r := zen.New(":0")
	r.Use(RequireAuth(&testAuth{
		user: &User{ID: "1", Username: "john", Claims: map[string]any{"tenant": "other"}},
	}))
	r.GET("/documents", RequireClaim("tenant", "acme"), func(c *zen.Ctx) {
		c.String(200, "ok")
	})

	req := httptest.NewRequest("GET", "/documents", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403", w.Code)
	}
}

func TestUser_GetClaim(t *testing.T) {
	user := &User{Claims: map[string]any{"tenant": "acme"}}
	value, ok := user.GetClaim("tenant")
	if !ok {
		t.Fatal("expected tenant claim to be found")
	}
	if value != "acme" {
		t.Fatalf("expected acme, got %v", value)
	}
}

func TestGetUser_Nil(t *testing.T) {
	r := zen.New(":0")
	r.GET("/no-auth", func(c *zen.Ctx) {
		u := GetUser(c)
		if u != nil {
			t.Fatal("GetUser should return nil when not authenticated")
		}
		c.String(200, "ok")
	})

	req := httptest.NewRequest("GET", "/no-auth", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
}

func TestUser_RequireRole(t *testing.T) {
	u := User{Authorities: []string{"role:admin", "role:user"}}

	if !u.RequireRole("admin") {
		t.Fatal("should have admin role")
	}
	if u.RequireRole("superadmin") {
		t.Fatal("should not have superadmin role")
	}
}

func TestUser_RequireRole_MixedCasePrefix(t *testing.T) {
	u := User{Authorities: []string{"ROLE:ADMIN", "ROLE:USER"}}

	if !u.RequireRole("ROLE:ADMIN") {
		t.Fatal("should match role authorities regardless of prefix casing")
	}
	if !u.RequireRole("admin") {
		t.Fatal("should match role authorities regardless of role casing")
	}
}

func TestUser_RequireAuthority(t *testing.T) {
	u := User{Authorities: []string{"read:documents"}}

	if !u.RequireAuthority("read:documents") {
		t.Fatal("should have authority")
	}
	if u.RequireAuthority("write:documents") {
		t.Fatal("should not have other authority")
	}
}

func TestUser_RequireAnyPermission(t *testing.T) {
	u := User{Authorities: []string{"read:documents"}}

	if !u.RequireAnyPermission("write:documents", "read:documents") {
		t.Fatal("should match any listed permission")
	}
	if u.RequireAnyPermission("write:documents", "delete:documents") {
		t.Fatal("should not match unrelated permissions")
	}
}

func TestUser_RequireAllPermissions(t *testing.T) {
	u := User{Authorities: []string{"read:documents", "write:documents"}}

	if !u.RequireAllPermissions("read:documents", "write:documents") {
		t.Fatal("should match all listed permissions")
	}
	if u.RequireAllPermissions("read:documents", "delete:documents") {
		t.Fatal("should require all permissions")
	}
}

func TestRequireAnyPermission(t *testing.T) {
	r := zen.New(":0")
	r.Use(RequireAuth(&testAuth{
		user: &User{ID: "1", Username: "john", Authorities: []string{"read:docs"}},
	}))
	r.GET("/docs", RequireAnyPermission("write:docs", "read:docs"), func(c *zen.Ctx) {
		c.String(200, "ok")
	})

	req := httptest.NewRequest("GET", "/docs", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
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

func TestValidatePassword_Bcrypt(t *testing.T) {
	hashed, err := HashPassword("secret123")
	if err != nil {
		t.Fatalf("HashPassword error: %v", err)
	}

	if !ValidatePassword(hashed, "secret123") {
		t.Fatal("should validate correct password")
	}
	if ValidatePassword(hashed, "wrong") {
		t.Fatal("should not validate wrong password")
	}
}

func TestValidatePassword_BcryptDirect(t *testing.T) {
	hashed, err := HashPassword("password")
	if err != nil {
		t.Fatalf("HashPassword error: %v", err)
	}
	if !ValidatePassword(hashed, "password") {
		t.Fatalf("should validate bcrypt hash")
	}
}

func TestValidatePassword_Empty(t *testing.T) {
	if ValidatePassword("", "secret") {
		t.Fatal("empty hash should not validate")
	}
	if ValidatePassword("hash", "") {
		t.Fatal("empty password should not validate")
	}
}

func TestValidatePassword_InvalidFormat(t *testing.T) {
	if ValidatePassword("nodelimiter", "secret") {
		t.Fatal("hash without delimiter should not validate")
	}
}

func TestJWT_GenerateAndParse(t *testing.T) {
	secret := []byte("test-secret")
	method := jwt.SigningMethodHS256
	claims := jwt.MapClaims{"sub": "123", "username": "john"}

	token, err := GenerateJWT(secret, method, claims, 1*time.Hour)
	if err != nil {
		t.Fatalf("GenerateJWT error: %v", err)
	}

	parsed, err := ParseJWT(token, secret, method)
	if err != nil {
		t.Fatalf("ParseJWT error: %v", err)
	}

	if parsed["sub"] != "123" {
		t.Fatalf("sub = %v, want 123", parsed["sub"])
	}
	if parsed["username"] != "john" {
		t.Fatalf("username = %v", parsed["username"])
	}
}

func TestJWT_ParseInvalid(t *testing.T) {
	_, err := ParseJWT("invalid.token.here", []byte("secret"), jwt.SigningMethodHS256)
	if err == nil {
		t.Fatal("should error for invalid token")
	}
}

func TestJWT_ParseWrongSecret(t *testing.T) {
	secret := []byte("test-secret")
	method := jwt.SigningMethodHS256

	token, err := GenerateJWT(secret, method, jwt.MapClaims{"sub": "1"}, 3600)
	if err != nil {
		t.Fatalf("GenerateJWT error: %v", err)
	}

	_, err = ParseJWT(token, []byte("wrong-secret"), method)
	if err == nil {
		t.Fatal("should error for wrong secret")
	}
}

func TestJWTAuth_Authenticate(t *testing.T) {
	secret := []byte("test-secret")
	method := jwt.SigningMethodHS256

	token, err := GenerateJWT(secret, method, jwt.MapClaims{"sub": "123", "username": "john"}, 1*time.Hour)
	if err != nil {
		t.Fatalf("GenerateJWT error: %v", err)
	}

	auth := &JWTAuth{
		Secret:        secret,
		SigningMethod: method,
	}

	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("Authorization", "Bearer "+token)

	user, err := auth.Authenticate(req)
	if err != nil {
		t.Fatalf("Authenticate error: %v", err)
	}
	if user.ID != "123" {
		t.Fatalf("id = %q, want %q", user.ID, "123")
	}
}

func TestJWTAuth_MissingHeader(t *testing.T) {
	auth := &JWTAuth{
		Secret:        []byte("secret"),
		SigningMethod: jwt.SigningMethodHS256,
	}

	req := httptest.NewRequest("GET", "/", nil)
	_, err := auth.Authenticate(req)
	if err == nil {
		t.Fatal("should error for missing header")
	}
}

func TestJWTAuth_Nil(t *testing.T) {
	var auth *JWTAuth
	req := httptest.NewRequest("GET", "/", nil)
	_, err := auth.Authenticate(req)
	if err == nil {
		t.Fatal("should error for nil auth")
	}
}

func TestBasicAuth_Authenticate(t *testing.T) {
	auth := &BasicAuth{
		Validate: func(username, password string) (*User, error) {
			if username == "john" && password == "secret" {
				return &User{ID: "1", Username: username}, nil
			}
			return nil, &testError{"invalid credentials"}
		},
	}

	req := httptest.NewRequest("GET", "/", nil)
	req.SetBasicAuth("john", "secret")

	user, err := auth.Authenticate(req)
	if err != nil {
		t.Fatalf("Authenticate error: %v", err)
	}
	if user.Username != "john" {
		t.Fatalf("username = %q, want %q", user.Username, "john")
	}
}

func TestBasicAuth_Invalid(t *testing.T) {
	auth := &BasicAuth{
		Validate: func(username, password string) (*User, error) {
			return nil, &testError{"invalid"}
		},
	}

	req := httptest.NewRequest("GET", "/", nil)
	req.SetBasicAuth("wrong", "creds")

	_, err := auth.Authenticate(req)
	if err == nil {
		t.Fatal("should error for invalid credentials")
	}
}

func TestBasicAuth_Missing(t *testing.T) {
	auth := &BasicAuth{
		Validate: func(username, password string) (*User, error) {
			return &User{ID: "1"}, nil
		},
	}

	req := httptest.NewRequest("GET", "/", nil)
	_, err := auth.Authenticate(req)
	if err == nil {
		t.Fatal("should error for missing basic auth")
	}
}

func TestBasicAuth_Challenge(t *testing.T) {
	auth := &BasicAuth{Realm: "myapp"}
	w := httptest.NewRecorder()
	auth.Challenge(w)

	header := w.Header().Get("WWW-Authenticate")
	if header == "" {
		t.Fatal("WWW-Authenticate header not set")
	}
}

func TestBasicAuth_Challenge_DefaultRealm(t *testing.T) {
	auth := &BasicAuth{}
	w := httptest.NewRecorder()
	auth.Challenge(w)

	header := w.Header().Get("WWW-Authenticate")
	if header == "" {
		t.Fatal("WWW-Authenticate header not set")
	}
}

func TestAPIKeyAuth_Header(t *testing.T) {
	auth := &APIKeyAuth{
		HeaderName: "X-API-Key",
		Validate: func(key string) (*User, error) {
			if key == "valid-key" {
				return &User{ID: "1"}, nil
			}
			return nil, &testError{"invalid key"}
		},
	}

	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("X-API-Key", "valid-key")

	user, err := auth.Authenticate(req)
	if err != nil {
		t.Fatalf("Authenticate error: %v", err)
	}
	if user.ID != "1" {
		t.Fatalf("id = %q, want %q", user.ID, "1")
	}
}

func TestAPIKeyAuth_QueryParam(t *testing.T) {
	auth := &APIKeyAuth{
		QueryParam: "api_key",
		Validate: func(key string) (*User, error) {
			if key == "valid-key" {
				return &User{ID: "1"}, nil
			}
			return nil, &testError{"invalid key"}
		},
	}

	req := httptest.NewRequest("GET", "/?api_key=valid-key", nil)

	user, err := auth.Authenticate(req)
	if err != nil {
		t.Fatalf("Authenticate error: %v", err)
	}
	if user.ID != "1" {
		t.Fatalf("id = %q, want %q", user.ID, "1")
	}
}

func TestAPIKeyAuth_Missing(t *testing.T) {
	auth := &APIKeyAuth{
		HeaderName: "X-API-Key",
		Validate: func(key string) (*User, error) {
			return &User{ID: "1"}, nil
		},
	}

	req := httptest.NewRequest("GET", "/", nil)
	_, err := auth.Authenticate(req)
	if err == nil {
		t.Fatal("should error for missing key")
	}
}

func TestAPIKeyAuth_Nil(t *testing.T) {
	var auth *APIKeyAuth
	req := httptest.NewRequest("GET", "/", nil)
	_, err := auth.Authenticate(req)
	if err == nil {
		t.Fatal("should error for nil auth")
	}
}

func TestSessionAuth(t *testing.T) {
	store := NewInMemorySessionStore(0) // no expiration
	store.Set("session123", &User{ID: "1", Username: "john"})

	auth := &SessionAuth{
		CookieName: "session_id",
		Store:      store,
	}

	req := httptest.NewRequest("GET", "/", nil)
	req.AddCookie(&http.Cookie{Name: "session_id", Value: "session123"})

	user, err := auth.Authenticate(req)
	if err != nil {
		t.Fatalf("Authenticate error: %v", err)
	}
	if user.Username != "john" {
		t.Fatalf("username = %q, want %q", user.Username, "john")
	}
}

func TestSessionAuth_MissingCookie(t *testing.T) {
	store := NewInMemorySessionStore(0)
	auth := &SessionAuth{
		CookieName: "session_id",
		Store:      store,
	}

	req := httptest.NewRequest("GET", "/", nil)
	_, err := auth.Authenticate(req)
	if err == nil {
		t.Fatal("should error for missing cookie")
	}
}

func TestSessionAuth_InvalidSession(t *testing.T) {
	store := NewInMemorySessionStore(0)
	auth := &SessionAuth{
		CookieName: "session_id",
		Store:      store,
	}

	req := httptest.NewRequest("GET", "/", nil)
	req.AddCookie(&http.Cookie{Name: "session_id", Value: "nonexistent"})

	_, err := auth.Authenticate(req)
	if err == nil {
		t.Fatal("should error for invalid session")
	}
}

func TestInMemorySessionStore(t *testing.T) {
	store := NewInMemorySessionStore(0) // no expiration
	user := &User{ID: "1", Username: "john"}

	store.Set("sess1", user)

	got, err := store.Get("sess1")
	if err != nil {
		t.Fatalf("Get error: %v", err)
	}
	if got.Username != "john" {
		t.Fatalf("username = %q, want %q", got.Username, "john")
	}

	_, err = store.Get("nonexistent")
	if err == nil {
		t.Fatal("should error for nonexistent session")
	}
}

func TestInMemorySessionStore_Expiration(t *testing.T) {
	store := NewInMemorySessionStore(50 * time.Millisecond)
	defer store.StopCleanup()
	user := &User{ID: "1", Username: "john"}

	store.Set("sess1", user)

	// Should work immediately
	_, err := store.Get("sess1")
	if err != nil {
		t.Fatalf("Get should succeed before expiration: %v", err)
	}

	// Wait for expiration
	time.Sleep(100 * time.Millisecond)

	// Should fail after expiration
	_, err = store.Get("sess1")
	if err == nil {
		t.Fatal("should error for expired session")
	}
}

func TestInMemorySessionStore_CleanupExpired(t *testing.T) {
	store := NewInMemorySessionStore(50 * time.Millisecond)
	defer store.StopCleanup()

	store.Set("sess1", &User{ID: "1"})
	store.Set("sess2", &User{ID: "2"})

	// Wait for expiration
	time.Sleep(100 * time.Millisecond)

	store.CleanupExpired()

	// Both should be cleaned up
	_, err1 := store.Get("sess1")
	_, err2 := store.Get("sess2")

	if err1 == nil || err2 == nil {
		t.Fatal("expired sessions should be cleaned up")
	}
}

func TestDefaultUserMapper(t *testing.T) {
	claims := jwt.MapClaims{
		"sub":      "123",
		"username": "john",
		"roles":    []any{"admin", "user"},
	}

	user := DefaultUserMapper(claims)
	if user == nil {
		t.Fatal("user should not be nil")
	}
	if user.ID != "123" {
		t.Fatalf("id = %q, want %q", user.ID, "123")
	}
	if user.Username != "john" {
		t.Fatalf("username = %q, want %q", user.Username, "john")
	}
	if len(user.Authorities) != 2 {
		t.Fatalf("authorities len = %d, want 2", len(user.Authorities))
	}
}

func TestDefaultUserMapper_NameFallback(t *testing.T) {
	claims := jwt.MapClaims{
		"sub":  "123",
		"name": "Jane",
	}

	user := DefaultUserMapper(claims)
	if user == nil {
		t.Fatal("user should not be nil")
	}
	if user.Username != "Jane" {
		t.Fatalf("username = %q, want %q", user.Username, "Jane")
	}
}

func TestDefaultUserMapper_ScopeRoles(t *testing.T) {
	claims := jwt.MapClaims{
		"sub":   "123",
		"scope": "read write admin",
	}

	user := DefaultUserMapper(claims)
	if user == nil {
		t.Fatal("user should not be nil")
	}
	if len(user.Authorities) != 3 {
		t.Fatalf("authorities len = %d, want 3", len(user.Authorities))
	}
}

func TestDefaultUserMapper_AuthoritiesRoles(t *testing.T) {
	claims := jwt.MapClaims{
		"sub":         "123",
		"authorities": []any{"ROLE_ADMIN", "ROLE_USER"},
	}

	user := DefaultUserMapper(claims)
	if user == nil {
		t.Fatal("user should not be nil")
	}
	if len(user.Authorities) != 2 {
		t.Fatalf("authorities len = %d, want 2", len(user.Authorities))
	}
	if user.Authorities[0] != "ROLE_ADMIN" {
		t.Fatalf("authorities[0] = %q, want %q", user.Authorities[0], "ROLE_ADMIN")
	}
}

func TestWithAuth(t *testing.T) {
	auth := &testAuth{user: &User{ID: "1"}}

	var handlerCalled bool
	handler := WithAuth(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handlerCalled = true
	}), auth)

	req := httptest.NewRequest("GET", "/", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if !handlerCalled {
		t.Fatal("handler not called")
	}
	if w.Code != 200 {
		t.Fatalf("status = %d, want 200", w.Code)
	}
}

func TestWithAuth_Failure(t *testing.T) {
	auth := &testAuth{err: errUnauth}

	var handlerCalled bool
	handler := WithAuth(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handlerCalled = true
	}), auth)

	req := httptest.NewRequest("GET", "/", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if handlerCalled {
		t.Fatal("handler should not be called")
	}
	if w.Code != 401 {
		t.Fatalf("status = %d, want 401", w.Code)
	}
}

func TestWithAuthFunc(t *testing.T) {
	auth := &testAuth{user: &User{ID: "1", Username: "john"}}

	var capturedUser *User
	handler := WithAuthFunc(func(w http.ResponseWriter, r *http.Request, user *User) {
		capturedUser = user
	}, auth)

	req := httptest.NewRequest("GET", "/", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if capturedUser.Username != "john" {
		t.Fatalf("username = %q, want %q", capturedUser.Username, "john")
	}
}

func TestMiddleware_Deprecated(t *testing.T) {
	r := zen.New(":0")
	r.Use(Middleware(&testAuth{
		user: &User{ID: "1"},
	}, nil))

	r.GET("/test", func(c *zen.Ctx) {
		c.String(200, "ok")
	})

	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("status = %d, want 200", w.Code)
	}
}

// --- Authority-based access control ---

func TestHasAuthority_Exists(t *testing.T) {
	u := User{Authorities: []string{"read:users"}}
	if !u.RequireAuthority("read:users") {
		t.Fatal("expected RequireAuthority to return true")
	}
}

func TestRequireAuthority_NotExists(t *testing.T) {
	u := User{Authorities: []string{"read:users"}}
	if u.RequireAuthority("write:admin") {
		t.Fatal("expected RequireAuthority to return false")
	}
}

func TestRequireAuthority_NilUser(t *testing.T) {
	var u User
	if u.RequireAuthority("anything") {
		t.Fatal("expected RequireAuthority to return false for nil user")
	}
}

func TestRequirePermission_Allowed(t *testing.T) {
	r := zen.New(":0")
	r.Use(RequireAuth(&testAuth{
		user: &User{
			ID: "1", Username: "john",
			Authorities: []string{"read:docs"},
		},
	}))
	r.GET("/docs", RequirePermission("read:docs"), func(c *zen.Ctx) {
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
	r := zen.New(":0")
	r.Use(RequireAuth(&testAuth{
		user: &User{
			ID: "1", Username: "john",
			Authorities: []string{"read:docs"},
		},
	}))
	r.GET("/admin", RequirePermission("admin:system"), func(c *zen.Ctx) {
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
	r := zen.New(":0")
	r.GET("/noauth", RequirePermission("read:anything"), func(c *zen.Ctx) {
		c.String(200, "ok")
	})

	req := httptest.NewRequest("GET", "/noauth", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != 403 {
		t.Fatalf("status = %d, want 403; body: %s", w.Code, w.Body.String())
	}
}
