package auth

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// Regression: the Bearer auth-scheme is case-insensitive per RFC 7235 5.1;
// lowercase "bearer" used to be rejected.
func TestJWTAuth_BearerSchemeCaseInsensitive(t *testing.T) {
	j := &JWTAuth{
		Secret:        []byte("test-secret"),
		SigningMethod: jwt.SigningMethodHS256,
	}

	token, err := j.Generate(jwt.MapClaims{"sub": "u1"}, time.Minute)
	if err != nil {
		t.Fatal(err)
	}

	for _, scheme := range []string{"Bearer", "bearer", "BEARER", "bEaReR"} {
		r := httptest.NewRequest(http.MethodGet, "/", nil)
		r.Header.Set("Authorization", scheme+" "+token)

		user, err := j.Authenticate(r)
		if err != nil {
			t.Errorf("scheme %q rejected: %v", scheme, err)
			continue
		}
		if user.ID != "u1" {
			t.Errorf("scheme %q: user.ID = %q, want u1", scheme, user.ID)
		}
	}
}

func TestJWTAuth_MissingOrMalformedHeader(t *testing.T) {
	j := &JWTAuth{Secret: []byte("k"), SigningMethod: jwt.SigningMethodHS256}

	if _, err := j.Authenticate(httptest.NewRequest("GET", "/", nil)); err == nil {
		t.Error("expected error for missing Authorization header")
	}

	r := httptest.NewRequest("GET", "/", nil)
	r.Header.Set("Authorization", "Bearer")
	if _, err := j.Authenticate(r); err == nil {
		t.Error("expected error for header without token")
	}
}

// Regression: non-string role entries used to leave empty holes in
// Authorities; entries must be compact and empty strings skipped.
func TestDefaultUserMapper_CompactAuthorities(t *testing.T) {
	user := DefaultUserMapper(jwt.MapClaims{
		"roles": []any{"admin", 42, "", nil, "editor"},
	})
	if len(user.Authorities) != 2 {
		t.Fatalf("Authorities = %#v, want [admin editor]", user.Authorities)
	}
	if user.Authorities[0] != "admin" || user.Authorities[1] != "editor" {
		t.Errorf("Authorities = %#v, want [admin editor]", user.Authorities)
	}
}

func TestDefaultUserMapper_AuthoritiesAndScope(t *testing.T) {
	u1 := DefaultUserMapper(jwt.MapClaims{"authorities": []any{"read", "write"}})
	if len(u1.Authorities) != 2 || u1.Authorities[0] != "read" {
		t.Errorf("Authorities = %#v", u1.Authorities)
	}

	u2 := DefaultUserMapper(jwt.MapClaims{"scope": "read write admin"})
	if len(u2.Authorities) != 3 || u2.Authorities[2] != "admin" {
		t.Errorf("scope mapping broken: %#v", u2.Authorities)
	}
}

// Regression: an empty authority string must never grant access, even if it
// is present in the authorities list.
func TestRequireAuthority_EmptyNeverGrants(t *testing.T) {
	u := &User{Authorities: []string{""}}
	if u.RequireAuthority("") {
		t.Error("empty authority must not match")
	}
	if (&User{}).RequireAuthority("") {
		t.Error("empty authority must not match")
	}
	var nilUser *User
	if nilUser.RequireAuthority("admin") {
		t.Error("nil user must not have authorities")
	}
}
