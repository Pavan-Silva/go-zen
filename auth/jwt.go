package auth

import (
	"errors"
	"fmt"
	"maps"
	"net/http"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// JWTAuth implements JWT token authentication. The Secret and SigningMethod
// fields are the single source of truth used both to verify incoming tokens
// (Authenticate) and to issue new ones (Generate).
type JWTAuth struct {
	Secret        []byte                           // Secret key used to verify token signatures.
	SigningMethod jwt.SigningMethod                // Expected signing method (e.g. jwt.SigningMethodHS256).
	ClaimsFunc    func(claims jwt.MapClaims) *User // Optional function to map JWT claims to a User pointer.
}

// validate reports whether the adapter carries the minimum configuration
// required to issue or verify tokens.
func (j *JWTAuth) validate() error {
	if j == nil {
		return errors.New("jwt auth is not configured")
	}
	if j.SigningMethod == nil {
		return errors.New("jwt signing method is not configured")
	}
	if len(j.Secret) == 0 {
		return errors.New("jwt secret is not configured")
	}
	return nil
}

// Authenticate extracts and validates a JWT token from the request.
func (j *JWTAuth) Authenticate(r *http.Request) (*User, error) {
	if err := j.validate(); err != nil {
		return nil, err
	}

	tokenString, err := bearerTokenFromRequest(r)
	if err != nil {
		return nil, err
	}

	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (any, error) {
		if token.Method.Alg() != j.SigningMethod.Alg() {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return j.Secret, nil
	})

	if err != nil {
		return nil, err
	}

	if claims, ok := token.Claims.(jwt.MapClaims); ok && token.Valid {
		return userFromClaims(j.ClaimsFunc, claims), nil
	}

	return nil, errors.New("invalid token")
}

// Generate creates a signed JWT containing the given claims, using the
// adapter's Secret and SigningMethod. The caller's map is not modified; iat
// and exp are derived from expiry on the copy, overwriting any same-named
// claims passed in.
func (j *JWTAuth) Generate(claims jwt.MapClaims, expiry time.Duration) (string, error) {
	if err := j.validate(); err != nil {
		return "", err
	}

	now := time.Now()

	// Copy claims to avoid modifying the caller's map
	claimsCopy := make(jwt.MapClaims, len(claims)+2)
	maps.Copy(claimsCopy, claims)

	claimsCopy["iat"] = now.Unix()
	claimsCopy["exp"] = now.Add(expiry).Unix()

	token := jwt.NewWithClaims(j.SigningMethod, claimsCopy)
	return token.SignedString(j.Secret)
}

// Parse verifies a token string using the adapter's Secret and SigningMethod
// and returns the claims.
func (j *JWTAuth) Parse(tokenString string) (jwt.MapClaims, error) {
	if err := j.validate(); err != nil {
		return nil, err
	}

	claims := jwt.MapClaims{}

	token, err := jwt.ParseWithClaims(tokenString, &claims, func(t *jwt.Token) (any, error) {
		if t.Method.Alg() != j.SigningMethod.Alg() {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		return j.Secret, nil
	})
	if err != nil {
		return nil, err
	}

	if !token.Valid {
		return nil, fmt.Errorf("invalid token")
	}

	return claims, nil
}

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

// DefaultUserMapper constructs a User from JWT claims.
func DefaultUserMapper(claims jwt.MapClaims) *User {
	user := &User{
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
		user.Authorities = stringAuthorities(roles)
	} else if roles, ok := claims["authorities"].([]any); ok {
		user.Authorities = stringAuthorities(roles)
	} else if scope, ok := claims["scope"].(string); ok && scope != "" {
		user.Authorities = strings.Split(scope, " ")
	}

	return user
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
