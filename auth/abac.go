package auth

import (
	"fmt"

	"github.com/Pavan-Silva/go-zen"
)

// GetClaim returns a claim value from the authenticated user.
func (u *User) GetClaim(key string) (any, bool) {
	if u == nil {
		return nil, false
	}
	if u.Claims == nil {
		return nil, false
	}
	val, ok := u.Claims[key]
	return val, ok
}

// RequireClaim creates middleware that requires a specific user claim value.
// It must be used after RequireAuth. Optionally pass a custom error handler.
func RequireClaim(key string, expected any, onError ...func(*zen.Ctx)) zen.HandlerFunc {
	return authorize(func(u *User) bool {
		value, exists := u.GetClaim(key)
		return exists && equalClaims(value, expected)
	}, onError...)
}

// equalClaims compares two claim values with type coercion.
// Uses fmt.Sprint to handle JSON-decoded float64 vs int literal comparisons.
func equalClaims(a, b any) bool {
	return fmt.Sprint(a) == fmt.Sprint(b)
}
