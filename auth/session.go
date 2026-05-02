package auth

import (
	"errors"
	"net/http"
	"sync"
	"time"
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
	if s == nil || s.Store == nil {
		return User{}, errors.New("session store is not configured")
	}
	if s.CookieName == "" {
		return User{}, errors.New("session cookie name is not configured")
	}

	cookie, err := r.Cookie(s.CookieName)
	if err != nil {
		return User{}, errors.New("missing session cookie")
	}

	return s.Store.Get(cookie.Value)
}

// sessionEntry holds a session value with an expiration time.
type sessionEntry struct {
	user      User
	expiresAt time.Time
}

// InMemorySessionStore is a simple in-memory session store for development.
// Sessions expire after the specified TTL and are cleaned up lazily.
type InMemorySessionStore struct {
	mu       sync.RWMutex
	sessions map[string]sessionEntry
	ttl      time.Duration
}

// NewInMemorySessionStore creates a new in-memory session store.
// ttl specifies how long sessions remain valid (e.g., 24*time.Hour).
// Pass 0 for no expiration (sessions never expire).
func NewInMemorySessionStore(ttl time.Duration) *InMemorySessionStore {
	return &InMemorySessionStore{
		sessions: make(map[string]sessionEntry),
		ttl:      ttl,
	}
}

// Get retrieves a session by ID. Returns error if not found or expired.
func (s *InMemorySessionStore) Get(sessionID string) (User, error) {
	s.mu.RLock()
	entry, ok := s.sessions[sessionID]
	s.mu.RUnlock()
	if !ok {
		return User{}, errors.New("invalid session")
	}
	if s.ttl > 0 && time.Now().After(entry.expiresAt) {
		s.mu.Lock()
		delete(s.sessions, sessionID)
		s.mu.Unlock()
		return User{}, errors.New("session expired")
	}
	return entry.user, nil
}

// Set stores a session. If ttl was configured, the session will expire after that duration.
func (s *InMemorySessionStore) Set(sessionID string, user User) {
	s.mu.Lock()
	var expiresAt time.Time
	if s.ttl > 0 {
		expiresAt = time.Now().Add(s.ttl)
	}
	s.sessions[sessionID] = sessionEntry{user: user, expiresAt: expiresAt}
	s.mu.Unlock()
}

// CleanupExpired removes all expired sessions. Call periodically or on a timer.
func (s *InMemorySessionStore) CleanupExpired() {
	if s.ttl == 0 {
		return
	}
	now := time.Now()
	s.mu.Lock()
	for k, v := range s.sessions {
		if now.After(v.expiresAt) {
			delete(s.sessions, k)
		}
	}
	s.mu.Unlock()
}
