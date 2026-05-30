package auth

import (
	"net/http"

	"github.com/Pavan-Silva/go-zen"
)

// Permissions builds a permission map from a list of permission strings.
//
//	user := &auth.User{
//	    Permissions: auth.Permissions("read:users", "write:reports"),
//	}
func Permissions(list ...string) map[string]struct{} {
	p := make(map[string]struct{}, len(list))
	for _, perm := range list {
		p[perm] = struct{}{}
	}
	return p
}

// HasPermission checks if the user has an explicit permission key.
func (u *User) HasPermission(permission string) bool {
	if u.Permissions == nil {
		return false
	}
	_, exists := u.Permissions[permission]
	return exists
}

// RequirePermission creates middleware that requires a specific granular permission.
// Must be chained after RequireAuth. Optionally accepts a custom error handler.
func RequirePermission(permission string, onError ...func(*zen.Ctx)) zen.HandlerFunc {
	errFunc := func(c *zen.Ctx) {
		c.Response.WriteHeader(http.StatusForbidden)
		_, _ = c.Response.Write([]byte(http.StatusText(http.StatusForbidden)))
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
		if !ok || !user.HasPermission(permission) {
			errFunc(c)
			return
		}

		c.Next()
	}
}
