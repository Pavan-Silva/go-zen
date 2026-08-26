package auth

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/Pavan-Silva/go-zen"
	"github.com/golang-jwt/jwt/v5"
)

// bearerTokenFromRequest extracts the Bearer token from the Authorization header.
// The auth-scheme is matched case-insensitively per RFC 7235 section 5.1
// (e.g. "Bearer", "bearer", and "BEARER" are all accepted).
func bearerTokenFromRequest(r *http.Request) (string, error) {
	ah := r.Header["Authorization"]
	if len(ah) == 0 {
		return "", fmt.Errorf("missing authorization header")
	}

	const scheme = "bearer "
	if len(ah[0]) > len(scheme) && strings.EqualFold(ah[0][:len(scheme)], scheme) {
		return ah[0][len(scheme):], nil
	}

	return "", fmt.Errorf("invalid authorization header format")
}

// userFromClaims maps JWT claims to a User using the optional claims function.
func userFromClaims(claimsFunc func(jwt.MapClaims) *User, claims jwt.MapClaims) *User {
	if claimsFunc != nil {
		return claimsFunc(claims)
	}
	return DefaultUserMapper(claims)
}

// stringAuthorities converts a heterogeneous claim array (e.g. JSON-decoded)
// into a compact []string, skipping non-string and empty entries instead of
// leaving empty holes that could match empty-authority checks.
func stringAuthorities(values []any) []string {
	authorities := make([]string, 0, len(values))
	for _, v := range values {
		if s, ok := v.(string); ok && s != "" {
			authorities = append(authorities, s)
		}
	}
	return authorities
}

// authorize returns middleware that denies the request with 403 via onError
// when no authenticated *User is present or check reports unauthorized.
func authorize(check func(*User) bool, onError ...func(*zen.Ctx)) zen.HandlerFunc {
	errFunc := func(c *zen.Ctx) {
		c.Error(http.StatusForbidden, http.StatusText(http.StatusForbidden))
	}
	if len(onError) > 0 && onError[0] != nil {
		errFunc = onError[0]
	}

	return func(c *zen.Ctx) {
		if user := GetUser(c); user == nil || !check(user) {
			errFunc(c)
			return
		}
		c.Next()
	}
}
