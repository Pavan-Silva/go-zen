package auth

import (
	"net/http"
	"slices"

	"github.com/Pavan-Silva/go-zen"
)

// HasRole checks if the user has a specific role.
func (u *User) HasRole(role string) bool {
	return slices.Contains(u.Roles, role)
}

// RequireRole creates middleware that requires a specific role.
// It must be used after RequireAuth. Optionally pass a custom error handler.
func RequireRole(role string, onError ...func(*zen.Ctx)) zen.HandlerFunc {
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
		if !ok || !user.HasRole(role) {
			errFunc(c)
			return
		}

		c.Next()
	}
}
