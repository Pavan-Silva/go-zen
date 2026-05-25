package auth

import (
	"errors"
	"fmt"
	"net/http"
)

// BasicAuth implements HTTP Basic Authentication.
type BasicAuth struct {
	Realm    string
	Validate func(username, password string) (*User, error) // UPDATED: Pointer signature contract
}

// Authenticate validates HTTP Basic Credentials.
func (b *BasicAuth) Authenticate(r *http.Request) (*User, error) {
	if b == nil || b.Validate == nil {
		return nil, errors.New("basic auth validator is not configured")
	}

	username, password, ok := r.BasicAuth()
	if !ok {
		return nil, errors.New("missing basic auth")
	}

	return b.Validate(username, password)
}

// Challenge sets the WWW-Authenticate header for a 401 response per RFC 7617.
// The realm identifies the protection space; if empty, defaults to "restricted".
func (b *BasicAuth) Challenge(w http.ResponseWriter) {
	realm := b.Realm
	if realm == "" {
		realm = "restricted"
	}
	w.Header().Set("WWW-Authenticate", fmt.Sprintf(`Basic realm=%q`, realm))
}

// APIKeyAuth implements API key authentication via header or query param.
type APIKeyAuth struct {
	HeaderName string                          // e.g., "X-API-Key"
	QueryParam string                          // e.g., "api_key"
	Validate   func(key string) (*User, error) // UPDATED: Pointer signature contract
}

// Authenticate validates the incoming token/key parameter.
func (a *APIKeyAuth) Authenticate(r *http.Request) (*User, error) {
	if a == nil || a.Validate == nil {
		return nil, errors.New("api key validator is not configured")
	}

	var key string

	if a.HeaderName != "" {
		key = r.Header.Get(a.HeaderName)
	}

	if key == "" && a.QueryParam != "" {
		key = r.URL.Query().Get(a.QueryParam)
	}

	if key == "" {
		return nil, errors.New("missing API key")
	}

	return a.Validate(key)
}
