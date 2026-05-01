package zen

import "net/http"

// SkipFunc decides whether a request should bypass middleware (auth, body limit, etc).
// It receives the current request and returns true for routes to skip.
//
// This type is defined in the root zen package so it can be shared
// across auth, middleware, and other packages without duplication.
//
// Example:
//
//	// Skip health checks from auth middleware
//	skip := func(r *http.Request) bool {
//	    return r.URL.Path == "/health"
//	}
//	r.Use(auth.RequireAuth(jwtAuth, skip))
type SkipFunc func(*http.Request) bool
