package auth

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/golang-jwt/jwt/v5"
)

// bearerTokenFromRequest extracts the Bearer token from the Authorization header.
// Returns an error if the header is missing or malformed.
func bearerTokenFromRequest(r *http.Request) (string, error) {
	authHeader := r.Header.Get("Authorization")
	if authHeader == "" {
		return "", fmt.Errorf("missing authorization header")
	}
	token := strings.TrimPrefix(authHeader, "Bearer ")
	if token == authHeader {
		return "", fmt.Errorf("invalid authorization header format")
	}
	return token, nil
}

// userFromClaims maps jwt.MapClaims to a User, either via a custom ClaimsFunc
// or the DefaultUserMapper.
func userFromClaims(claimsFunc func(jwt.MapClaims) User, claims jwt.MapClaims) User {
	if claimsFunc != nil {
		return claimsFunc(claims)
	}
	return DefaultUserMapper(claims)
}
