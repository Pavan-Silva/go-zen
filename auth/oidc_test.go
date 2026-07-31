package auth

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/golang-jwt/jwt/v5"
)

func TestOIDCAuth_Success(t *testing.T) {
	userinfoServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" {
			t.Fatalf("method = %s, want GET", r.Method)
		}
		if r.Header.Get("Authorization") != "Bearer valid-token" {
			t.Fatalf("Authorization = %q", r.Header.Get("Authorization"))
		}
		if r.Header.Get("Accept") != "application/json" {
			t.Fatalf("Accept = %q", r.Header.Get("Accept"))
		}
		info := OIDCUserInfo{
			Sub:               "user-1",
			Name:              "John Doe",
			PreferredUsername: "johnd",
			Email:             "john@example.com",
			EmailVerified:     true,
			GivenName:         "John",
			FamilyName:        "Doe",
			Picture:           "https://example.com/avatar.jpg",
			Locale:            "en-US",
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(info)
	}))
	defer userinfoServer.Close()

	auth := &OIDCAuth{
		Issuer:                "https://accounts.example.com",
		ClientID:              "my-client",
		UserInfoEndpoint:      userinfoServer.URL,
		SkipTokenVerification: true,
	}

	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("Authorization", "Bearer valid-token")

	user, err := auth.Authenticate(req)
	if err != nil {
		t.Fatalf("Authenticate() error = %v", err)
	}
	if user.ID != "user-1" {
		t.Fatalf("user.ID = %q, want %q", user.ID, "user-1")
	}
	if user.Username != "John Doe" {
		t.Fatalf("user.Username = %q, want %q", user.Username, "John Doe")
	}
}

func TestOIDCAuth_NoBearer(t *testing.T) {
	auth := &OIDCAuth{
		Issuer:           "https://accounts.example.com",
		UserInfoEndpoint: "http://example.com/userinfo",
	}

	req := httptest.NewRequest("GET", "/", nil)
	_, err := auth.Authenticate(req)
	if err == nil {
		t.Fatal("expected error for missing bearer token")
	}
}

func TestOIDCAuth_Nil(t *testing.T) {
	var auth *OIDCAuth
	req := httptest.NewRequest("GET", "/", nil)
	_, err := auth.Authenticate(req)
	if err == nil {
		t.Fatal("expected error for nil auth")
	}
}

func TestOIDCAuth_UserInfoFails(t *testing.T) {
	userinfoServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer userinfoServer.Close()

	auth := &OIDCAuth{
		UserInfoEndpoint:      userinfoServer.URL,
		SkipTokenVerification: true,
	}

	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("Authorization", "Bearer bad-token")

	_, err := auth.Authenticate(req)
	if err == nil {
		t.Fatal("expected error for failed userinfo request")
	}
}

func TestOIDCAuth_DefaultEndpoint(t *testing.T) {
	var called bool
	userinfoServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		info := OIDCUserInfo{Sub: "user-1"}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(info)
	}))
	defer userinfoServer.Close()

	auth := &OIDCAuth{
		Issuer:                userinfoServer.URL,
		UserInfoEndpoint:      "",
		SkipTokenVerification: true,
	}

	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("Authorization", "Bearer token")

	_, err := auth.Authenticate(req)
	if err != nil {
		t.Fatalf("Authenticate() error = %v", err)
	}

	// Should have called issuer + /oauth2/v2/userinfo
	if !called {
		t.Fatal("expected userinfo call with default endpoint")
	}
}

func TestOIDCAuth_CustomClaimsFunc(t *testing.T) {
	userinfoServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		info := OIDCUserInfo{
			Sub:  "user-1",
			Name: "John",
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(info)
	}))
	defer userinfoServer.Close()

	auth := &OIDCAuth{
		UserInfoEndpoint:      userinfoServer.URL,
		SkipTokenVerification: true,
		ClaimsFunc: func(claims jwt.MapClaims) *User {
			return &User{
				ID:       "oidc-" + claims["sub"].(string),
				Username: claims["name"].(string),
			}
		},
	}

	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("Authorization", "Bearer token")

	user, err := auth.Authenticate(req)
	if err != nil {
		t.Fatalf("Authenticate() error = %v", err)
	}
	if user.ID != "oidc-user-1" {
		t.Fatalf("user.ID = %q, want %q", user.ID, "oidc-user-1")
	}
}

func TestOIDCAuth_VerificationFailsClosedWithoutKeyFunc(t *testing.T) {
	auth := &OIDCAuth{
		UserInfoEndpoint: "http://example.com/userinfo",
	}

	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("Authorization", "Bearer any-token")

	if _, err := auth.Authenticate(req); err == nil {
		t.Fatal("expected error when SkipTokenVerification is false and no KeyFunc is configured")
	} else if !errors.Is(err, ErrOIDCKeyFuncNotConfigured) {
		t.Fatalf("error = %v, want ErrOIDCKeyFuncNotConfigured", err)
	}
}

func TestOIDCAuth_VerificationRejectsForgedToken(t *testing.T) {
	auth := &OIDCAuth{
		UserInfoEndpoint: "http://example.com/userinfo",
		KeyFunc: func(t *jwt.Token) (any, error) {
			return []byte("secret"), nil
		},
	}

	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("Authorization", "Bearer not-a-real-signature")

	if _, err := auth.Authenticate(req); err == nil {
		t.Fatal("expected forged token to be rejected")
	}
}

func TestOIDCAuth_VerificationAcceptsValidToken(t *testing.T) {
	userinfoServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		info := OIDCUserInfo{Sub: "user-1"}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(info)
	}))
	defer userinfoServer.Close()

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"sub": "user-1",
		"iss": "https://accounts.example.com",
	})
	signed, err := token.SignedString([]byte("secret"))
	if err != nil {
		t.Fatalf("SignedString() error = %v", err)
	}

	auth := &OIDCAuth{
		UserInfoEndpoint: userinfoServer.URL,
		KeyFunc: func(t *jwt.Token) (any, error) {
			return []byte("secret"), nil
		},
	}

	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("Authorization", "Bearer "+signed)

	user, err := auth.Authenticate(req)
	if err != nil {
		t.Fatalf("Authenticate() error = %v", err)
	}
	if user.ID != "user-1" {
		t.Fatalf("user.ID = %q, want %q", user.ID, "user-1")
	}
}

func TestOIDCAuth_SkipTokenVerification(t *testing.T) {
	userinfoServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		info := OIDCUserInfo{
			Sub:  "user-1",
			Name: "John",
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(info)
	}))
	defer userinfoServer.Close()

	auth := &OIDCAuth{
		UserInfoEndpoint:      userinfoServer.URL,
		SkipTokenVerification: true,
	}

	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("Authorization", "Bearer opaque-token")

	user, err := auth.Authenticate(req)
	if err != nil {
		t.Fatalf("Authenticate() error = %v", err)
	}
	if user.ID != "user-1" {
		t.Fatalf("user.ID = %q, want %q", user.ID, "user-1")
	}
}
