package auth

import (
	"errors"
	"net/http"
)

// SessionAuth implements session-based authentication using cookies.
type SessionAuth struct {
	CookieName string
	Store      SessionStore
}

type SessionStore interface {
	Get(sessionID string) (User, error)
}

func (s *SessionAuth) Authenticate(r *http.Request) (User, error) {
	cookie, err := r.Cookie(s.CookieName)
	if err != nil {
		return User{}, errors.New("missing session cookie")
	}

	return s.Store.Get(cookie.Value)
}

// InMemorySessionStore is a simple in-memory session store for development.
type InMemorySessionStore struct {
	sessions map[string]User
}

func NewInMemorySessionStore() *InMemorySessionStore {
	return &InMemorySessionStore{
		sessions: make(map[string]User),
	}
}

func (s *InMemorySessionStore) Get(sessionID string) (User, error) {
	user, ok := s.sessions[sessionID]
	if !ok {
		return User{}, errors.New("invalid session")
	}
	return user, nil
}

func (s *InMemorySessionStore) Set(sessionID string, user User) {
	s.sessions[sessionID] = user
}
