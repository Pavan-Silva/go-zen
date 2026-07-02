package auth

import (
	"fmt"
	"net/http"

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
	errFunc := func(c *zen.Ctx) {
		c.Error(http.StatusForbidden, http.StatusText(http.StatusForbidden))
	}
	if len(onError) > 0 && onError[0] != nil {
		errFunc = onError[0]
	}

	return func(c *zen.Ctx) {
		userVal, ok := c.Get("user")
		if !ok || userVal == nil {
			errFunc(c)
			return
		}

		user, ok := userVal.(*User)
		if !ok || user == nil {
			errFunc(c)
			return
		}

		value, exists := user.GetClaim(key)
		if !exists || !equalClaims(value, expected) {
			errFunc(c)
			return
		}

		c.Next()
	}
}

// equalClaims compares two claim values with type coercion.
// Uses fmt.Sprint to handle JSON-decoded float64 vs int literal comparisons.
func equalClaims(a, b any) bool {
	return fmt.Sprint(a) == fmt.Sprint(b)
}
