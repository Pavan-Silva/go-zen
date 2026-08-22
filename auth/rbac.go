package auth

import (
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
	return authorize(func(u *User) bool { return u.RequireRole(role) }, onError...)
}
