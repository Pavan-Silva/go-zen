package auth

import (
	"errors"
	"net/http"
)

// APIKeyAuth implements API key authentication via header or query param.
type APIKeyAuth struct {
	HeaderName string // e.g., "X-API-Key"
	QueryParam string // e.g., "api_key"
	Validate   func(key string) (User, error)
}

func (a *APIKeyAuth) Authenticate(r *http.Request) (User, error) {
	var key string

	if a.HeaderName != "" {
		key = r.Header.Get(a.HeaderName)
	}

	if key == "" && a.QueryParam != "" {
		key = r.URL.Query().Get(a.QueryParam)
	}

	if key == "" {
		return User{}, errors.New("missing API key")
	}

	return a.Validate(key)
}
