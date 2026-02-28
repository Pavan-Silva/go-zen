package zen

import (
    "net/http"
)

// HandlerFunc defines the signature for zen handlers. It mirrors http.HandlerFunc
// but with a *Context argument so utilities are available. This file is separate
// mostly to keep context.go trimmed and to mimic the multi-file structure of
// larger frameworks like Gin or Echo.

type HandlerFunc func(*Context)

// Adapt converts a zen.HandlerFunc into an http.HandlerFunc so it can be
// registered with the standard library's mux. It simply constructs a fresh
// *Context per request; the allocation cost is extremely low and the compiler
// can often elide it.
func Adapt(h HandlerFunc) http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        ctx := &Context{Response: w, Request: r}
        h(ctx)
    }
}
