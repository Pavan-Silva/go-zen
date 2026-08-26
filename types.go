package zen

import "net/http"

// HandlerFunc is the universal type for both route handlers and middleware.
type HandlerFunc func(*Ctx)

// SkipFunc decides whether a request should bypass middleware (auth, body limit, etc.).
// It receives the current request and returns true for routes to skip.
type SkipFunc func(*http.Request) bool
