package auth

import (
	"errors"
	"net/http"
	"sync"
	"time"
)

// SessionAuth implements session-based authentication using cookies.
type SessionAuth struct {
	CookieName string       // Name of the cookie that holds the session ID.
	Store      SessionStore // Backing store used to look up sessions.
}

// SessionStore looks up sessions by ID.
type SessionStore interface {
	Get(sessionID string) (*User, error)
}

// Authenticate validates the session cookie and returns the user.
func (s *SessionAuth) Authenticate(r *http.Request) (*User, error) {
	if s == nil || s.Store == nil {
		return nil, errors.New("session store is not configured")
	}

	cookieName := s.CookieName
	if cookieName == "" {
		cookieName = "zen-session"
	}

	cookie, err := r.Cookie(cookieName)
	if err != nil {
		return nil, errors.New("missing session cookie")
	}

	return s.Store.Get(cookie.Value)
}

// sessionEntry holds a user session with an expiration time.
type sessionEntry struct {
	user      *User
	expiresAt time.Time
}

// InMemorySessionStore is a simple in-memory session store for development.
type InMemorySessionStore struct {
	mu       sync.RWMutex
	sessions map[string]sessionEntry
	ttl      time.Duration
}

// NewInMemorySessionStore creates a new in-memory session store.
func NewInMemorySessionStore(ttl time.Duration) *InMemorySessionStore {
	store := &InMemorySessionStore{
		sessions: make(map[string]sessionEntry),
		ttl:      ttl,
	}
	if ttl > 0 {
		go func() {
			ticker := time.NewTicker(time.Minute)
			for range ticker.C {
				store.CleanupExpired()
			}
		}()
	}
	return store
}

// Get retrieves a session by ID.
func (s *InMemorySessionStore) Get(sessionID string) (*User, error) {
	s.mu.RLock()
	entry, ok := s.sessions[sessionID]
	s.mu.RUnlock()
	if !ok {
		return nil, errors.New("invalid session")
	}
	if s.ttl > 0 && time.Now().After(entry.expiresAt) {
		s.mu.Lock()
		delete(s.sessions, sessionID)
		s.mu.Unlock()
		return nil, errors.New("session expired")
	}
	return entry.user, nil
}

// Set stores a session for the given session ID.
func (s *InMemorySessionStore) Set(sessionID string, user *User) {
	s.mu.Lock()
	var expiresAt time.Time
	if s.ttl > 0 {
		expiresAt = time.Now().Add(s.ttl)
	}
	s.sessions[sessionID] = sessionEntry{user: user, expiresAt: expiresAt}
	s.mu.Unlock()
}

// CleanupExpired removes all expired sessions.
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
