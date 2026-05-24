package zen

// HandlerFunc is the universal type for both route handlers and middleware.
type HandlerFunc func(*Ctx)
