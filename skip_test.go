package zen

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestSkipFunc_Type(t *testing.T) {
	var skip SkipFunc = func(r *http.Request) bool {
		return r.URL.Path == "/health"
	}

	req := httptest.NewRequest("GET", "/health", nil)
	if !skip(req) {
		t.Fatal("skip should return true for /health")
	}

	req = httptest.NewRequest("GET", "/users", nil)
	if skip(req) {
		t.Fatal("skip should return false for /users")
	}
}
