package auth

import (
	"errors"
	"net/http"
	"sync"
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
	mu       sync.RWMutex
	sessions map[string]User
}

func NewInMemorySessionStore() *InMemorySessionStore {
	return &InMemorySessionStore{
		sessions: make(map[string]User),
	}
}

func (s *InMemorySessionStore) Get(sessionID string) (User, error) {
	s.mu.RLock()
	user, ok := s.sessions[sessionID]
	s.mu.RUnlock()
	if !ok {
		return User{}, errors.New("invalid session")
	}
	return user, nil
}

func (s *InMemorySessionStore) Set(sessionID string, user User) {
	s.mu.Lock()
	s.sessions[sessionID] = user
	s.mu.Unlock()
}
