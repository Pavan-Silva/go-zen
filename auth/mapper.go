package auth

import (
	"strings"

	"github.com/golang-jwt/jwt/v5"
)

// DefaultUserMapper maps JWT claims to User struct.
// Supports standard claims: "sub" for ID, "username" for Username, "roles" or "authorities" or "scope" for Roles.
func DefaultUserMapper(claims jwt.MapClaims) User {
	user := User{
		Claims: claims,
	}

	if sub, ok := claims["sub"].(string); ok {
		user.ID = sub
	}

	if username, ok := claims["username"].(string); ok {
		user.Username = username
	} else if name, ok := claims["name"].(string); ok {
		user.Username = name
	}

	if roles, ok := claims["roles"].([]any); ok {
		user.Roles = make([]string, len(roles))
		for i, r := range roles {
			if s, ok := r.(string); ok {
				user.Roles[i] = s
			}
		}
	} else if roles, ok := claims["authorities"].([]any); ok {
		user.Roles = make([]string, len(roles))
		for i, r := range roles {
			if s, ok := r.(string); ok {
				user.Roles[i] = s
			}
		}
	} else if scope, ok := claims["scope"].(string); ok && scope != "" {
		user.Roles = strings.Split(scope, " ")
	}

	return user
}
