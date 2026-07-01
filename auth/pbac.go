package auth

import (
	"net/http"
	"slices"

	"github.com/Pavan-Silva/go-zen"
)

// Authorities builds a slice of authority strings.
func Authorities(list ...string) []string {
	return append([]string(nil), list...)
}

// RequireAuthority checks if the user has a raw authority string.
func (u *User) RequireAuthority(authority string) bool {
	if u == nil {
		return false
	}
	return slices.Contains(u.Authorities, authority)
}

// RequireAnyPermission checks whether the user has at least one of the supplied permissions.
func (u *User) RequireAnyPermission(permissions ...string) bool {
	if u == nil {
		return false
	}
	for _, permission := range permissions {
		if permission != "" && u.RequireAuthority(permission) {
			return true
		}
	}
	return false
}

// RequireAllPermissions checks whether the user has every supplied permission.
func (u *User) RequireAllPermissions(permissions ...string) bool {
	if u == nil {
		return false
	}
	if len(permissions) == 0 {
		return false
	}
	for _, permission := range permissions {
		if permission == "" || !u.RequireAuthority(permission) {
			return false
		}
	}
	return true
}

// RequirePermission creates middleware that requires a specific permission-like authority.
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
		if !ok || !user.RequireAuthority(permission) {
			errFunc(c)
			return
		}

		c.Next()
	}
}

// RequireAnyPermission creates middleware that requires at least one of the supplied permissions.
func RequireAnyPermission(permissions ...string) zen.HandlerFunc {
	return func(c *zen.Ctx) {
		userVal, ok := c.Get("user")
		if !ok || userVal == nil {
			c.Response.WriteHeader(http.StatusForbidden)
			_, _ = c.Response.Write([]byte(http.StatusText(http.StatusForbidden)))
			return
		}

		user, ok := userVal.(*User)
		if !ok || !user.RequireAnyPermission(permissions...) {
			c.Response.WriteHeader(http.StatusForbidden)
			_, _ = c.Response.Write([]byte(http.StatusText(http.StatusForbidden)))
			return
		}

		c.Next()
	}
}

// RequireAllPermissions creates middleware that requires every supplied permission.
func RequireAllPermissions(permissions ...string) zen.HandlerFunc {
	return func(c *zen.Ctx) {
		userVal, ok := c.Get("user")
		if !ok || userVal == nil {
			c.Response.WriteHeader(http.StatusForbidden)
			_, _ = c.Response.Write([]byte(http.StatusText(http.StatusForbidden)))
			return
		}

		user, ok := userVal.(*User)
		if !ok || !user.RequireAllPermissions(permissions...) {
			c.Response.WriteHeader(http.StatusForbidden)
			_, _ = c.Response.Write([]byte(http.StatusText(http.StatusForbidden)))
			return
		}

		c.Next()
	}
}
