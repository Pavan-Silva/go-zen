package auth

import (
	"errors"
	"net/http"
)

// BasicAuth implements HTTP Basic Authentication.
type BasicAuth struct {
	Realm    string
	Validate func(username, password string) (User, error)
}

func (b *BasicAuth) Authenticate(r *http.Request) (User, error) {
	username, password, ok := r.BasicAuth()
	if !ok {
		return User{}, errors.New("missing basic auth")
	}

	return b.Validate(username, password)
}
