package auth

import (
	"net/http"
	"strings"

	"github.com/Pavan-Silva/go-zen"
)

// RequireRole checks if the user has a specific role.
// Roles are matched case-insensitively using the standard role:NAME format.
func (u *User) RequireRole(role string) bool {
	if u == nil || role == "" {
		return false
	}

	normalizedRole := normalizeRole(role)
	for _, authority := range u.Authorities {
		if normalizeRole(authority) == normalizedRole {
			return true
		}
	}

	return false
}

func normalizeRole(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}

	upperValue := strings.ToUpper(value)
	if after, ok := strings.CutPrefix(upperValue, "ROLE:"); ok {
		return after
	}

	return upperValue
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
		if !ok || !user.RequireRole(role) {
			errFunc(c)
			return
		}

		c.Next()
	}
}
