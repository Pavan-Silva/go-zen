package auth

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Pavan-Silva/go-zen"
	"github.com/golang-jwt/jwt/v5"
)

type testError struct {
	msg string
}

func (e *testError) Error() string {
	return e.msg
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
	cfg := &JWTAuth{
		Secret:        []byte("test-secret"),
		SigningMethod: jwt.SigningMethodHS256,
	}
	claims := jwt.MapClaims{"sub": "123", "username": "john"}

	token, err := cfg.Generate(claims, 1*time.Hour)
	if err != nil {
		t.Fatalf("Generate error: %v", err)
	}

	parsed, err := cfg.Parse(token)
	if err != nil {
		t.Fatalf("Parse error: %v", err)
	}

	if parsed["sub"] != "123" {
		t.Fatalf("sub = %v, want 123", parsed["sub"])
	}
	if parsed["username"] != "john" {
		t.Fatalf("username = %v", parsed["username"])
	}
}

func TestJWT_ParseInvalid(t *testing.T) {
	cfg := &JWTAuth{
		Secret:        []byte("secret"),
		SigningMethod: jwt.SigningMethodHS256,
	}

	_, err := cfg.Parse("invalid.token.here")
	if err == nil {
		t.Fatal("should error for invalid token")
	}
}

func TestJWT_ParseWrongSecret(t *testing.T) {
	signer := &JWTAuth{
		Secret:        []byte("test-secret"),
		SigningMethod: jwt.SigningMethodHS256,
	}

	token, err := signer.Generate(jwt.MapClaims{"sub": "1"}, time.Hour)
	if err != nil {
		t.Fatalf("Generate error: %v", err)
	}

	verifier := &JWTAuth{
		Secret:        []byte("wrong-secret"),
		SigningMethod: jwt.SigningMethodHS256,
	}
	if _, err = verifier.Parse(token); err == nil {
		t.Fatal("should error for wrong secret")
	}
}

func TestJWT_ConfigValidation(t *testing.T) {
	var nilCfg *JWTAuth
	if _, err := nilCfg.Generate(jwt.MapClaims{}, time.Hour); err == nil {
		t.Fatal("nil adapter should error on Generate")
	}
	if _, err := nilCfg.Parse("x"); err == nil {
		t.Fatal("nil adapter should error on Parse")
	}

	cfg := &JWTAuth{}
	if _, err := cfg.Generate(jwt.MapClaims{}, time.Hour); err == nil {
		t.Fatal("missing signing method should error")
	}

	cfg.SigningMethod = jwt.SigningMethodHS256
	if _, err := cfg.Generate(jwt.MapClaims{}, time.Hour); err == nil {
		t.Fatal("missing secret should error")
	}
}

func TestJWTAuth_Authenticate(t *testing.T) {
	auth := &JWTAuth{
		Secret:        []byte("test-secret"),
		SigningMethod: jwt.SigningMethodHS256,
	}

	token, err := auth.Generate(jwt.MapClaims{"sub": "123", "username": "john"}, 1*time.Hour)
	if err != nil {
		t.Fatalf("Generate error: %v", err)
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
		Validate: func(username, password string) (*zen.User, error) {
			if username == "john" && password == "secret" {
				return &zen.User{ID: "1", Username: username}, nil
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
		Validate: func(username, password string) (*zen.User, error) {
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
		Validate: func(username, password string) (*zen.User, error) {
			return &zen.User{ID: "1"}, nil
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
		Validate: func(key string) (*zen.User, error) {
			if key == "valid-key" {
				return &zen.User{ID: "1"}, nil
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
		Validate: func(key string) (*zen.User, error) {
			if key == "valid-key" {
				return &zen.User{ID: "1"}, nil
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
		Validate: func(key string) (*zen.User, error) {
			return &zen.User{ID: "1"}, nil
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
	store.Set("session123", &zen.User{ID: "1", Username: "john"})

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
	user := &zen.User{ID: "1", Username: "john"}

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
	user := &zen.User{ID: "1", Username: "john"}

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

	store.Set("sess1", &zen.User{ID: "1"})
	store.Set("sess2", &zen.User{ID: "2"})

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

	user := defaultUserMapper(claims)
	if user == nil {
		t.Fatal("user should not be nil")
	}
	if user.ID != "123" {
		t.Fatalf("id = %q, want %q", user.ID, "123")
	}
	if user.Username != "john" {
		t.Fatalf("username = %q, want %q", user.Username, "john")
	}
	if len(user.Roles) != 2 {
		t.Fatalf("roles len = %d, want 2", len(user.Roles))
	}
}

func TestDefaultUserMapper_NameFallback(t *testing.T) {
	claims := jwt.MapClaims{
		"sub":  "123",
		"name": "Jane",
	}

	user := defaultUserMapper(claims)
	if user == nil {
		t.Fatal("user should not be nil")
	}
	if user.Username != "Jane" {
		t.Fatalf("username = %q, want %q", user.Username, "Jane")
	}
}

func TestDefaultUserMapper_ScopeNotMapped(t *testing.T) {
	claims := jwt.MapClaims{
		"sub":   "123",
		"scope": "read write admin",
	}

	user := defaultUserMapper(claims)
	if user == nil {
		t.Fatal("user should not be nil")
	}
	if len(user.Roles) != 0 {
		t.Fatalf("scope must not populate roles, got %#v", user.Roles)
	}
}

func TestDefaultUserMapper_AuthoritiesNotMapped(t *testing.T) {
	claims := jwt.MapClaims{
		"sub":         "123",
		"authorities": []any{"ROLE_ADMIN", "ROLE_USER"},
	}

	user := defaultUserMapper(claims)
	if user == nil {
		t.Fatal("user should not be nil")
	}
	if len(user.Roles) != 0 {
		t.Fatalf("authorities claim must not populate roles, got %#v", user.Roles)
	}
}

func TestDefaultUserMapper_SingleRoleClaim(t *testing.T) {
	user := defaultUserMapper(jwt.MapClaims{
		"sub":  "123",
		"role": "admin",
	})
	if user == nil {
		t.Fatal("user should not be nil")
	}
	if len(user.Roles) != 1 || user.Roles[0] != "admin" {
		t.Fatalf("roles = %#v, want [admin]", user.Roles)
	}
}
