package auth

import (
	"slices"

	"github.com/Pavan-Silva/go-zen"
)

// RequireAuthority checks if the user has a raw authority string.
// An empty authority never grants access.
func (u *User) RequireAuthority(authority string) bool {
	if u == nil || authority == "" {
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
	return authorize(func(u *User) bool { return u.RequireAuthority(permission) }, onError...)
}

// RequireAnyPermission creates middleware that requires at least one of the supplied permissions.
func RequireAnyPermission(permissions ...string) zen.HandlerFunc {
	return authorize(func(u *User) bool { return u.RequireAnyPermission(permissions...) })
}

// RequireAllPermissions creates middleware that requires every supplied permission.
func RequireAllPermissions(permissions ...string) zen.HandlerFunc {
	return authorize(func(u *User) bool { return u.RequireAllPermissions(permissions...) })
}
