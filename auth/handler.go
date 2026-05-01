package auth

import "net/http"

// WithAuth wraps an http.Handler to require authentication before serving.
// Returns 401 if authentication fails. For use with WebSocket upgrades.
func WithAuth(handler http.Handler, auth Authenticator) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if _, err := auth.Authenticate(r); err != nil {
			http.Error(w, http.StatusText(http.StatusUnauthorized), http.StatusUnauthorized)
			return
		}
		handler.ServeHTTP(w, r)
	})
}

// WithAuthFunc wraps an http.HandlerFunc to require authentication before serving.
// The authenticated User is passed to the handler. For use with WebSocket upgrades.
func WithAuthFunc(handler func(w http.ResponseWriter, r *http.Request, user User), auth Authenticator) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user, err := auth.Authenticate(r)
		if err != nil {
			http.Error(w, http.StatusText(http.StatusUnauthorized), http.StatusUnauthorized)
			return
		}
		handler(w, r, user)
	})
}
