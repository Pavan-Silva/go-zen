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

// Regression: non-string role entries used to leave empty holes in the role
// list; entries must be compact, empty strings skipped, and role names kept
// verbatim (no prefix normalization).
func TestDefaultUserMapper_RolesCompact(t *testing.T) {
	user := defaultUserMapper(jwt.MapClaims{
		"roles": []any{"admin", 42, "", nil, "editor", "ROLE:prefixed"},
	})
	want := []string{"admin", "editor", "ROLE:prefixed"}
	if len(user.Roles) != len(want) {
		t.Fatalf("Roles = %#v, want %#v", user.Roles, want)
	}
	for i := range want {
		if user.Roles[i] != want[i] {
			t.Errorf("Roles = %#v, want %#v", user.Roles, want)
		}
	}
}

func TestDefaultUserMapper_AuthoritiesAndScopeIgnored(t *testing.T) {
	u1 := defaultUserMapper(jwt.MapClaims{"authorities": []any{"read", "write"}})
	if len(u1.Roles) != 0 {
		t.Errorf("authorities claim must not map to roles: %#v", u1.Roles)
	}

	u2 := defaultUserMapper(jwt.MapClaims{"scope": "read write admin"})
	if len(u2.Roles) != 0 {
		t.Errorf("scope must not map to roles: %#v", u2.Roles)
	}
}
