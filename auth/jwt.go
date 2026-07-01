package auth

import (
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// JWTAuth implements JWT token authentication.
type JWTAuth struct {
	Secret        []byte                           // Secret key used to verify token signatures.
	SigningMethod jwt.SigningMethod                // Expected signing method (e.g. jwt.SigningMethodHS256).
	ClaimsFunc    func(claims jwt.MapClaims) *User // Optional function to map JWT claims to a User pointer.
}

// Authenticate extracts and validates a JWT token from the request.
func (j *JWTAuth) Authenticate(r *http.Request) (*User, error) {
	if j == nil {
		return nil, errors.New("jwt auth is not configured")
	}
	if j.SigningMethod == nil {
		return nil, errors.New("jwt signing method is not configured")
	}
	if len(j.Secret) == 0 {
		return nil, errors.New("jwt secret is not configured")
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

// GenerateJWT creates a JWT token with the given claims.
func GenerateJWT(secret []byte, method jwt.SigningMethod, claims jwt.MapClaims, expiry time.Duration) (string, error) {
	now := time.Now()

	// Copy claims to avoid modifying the caller's map
	claimsCopy := make(jwt.MapClaims, len(claims)+2)
	for k, v := range claims {
		claimsCopy[k] = v
	}
	claimsCopy["iat"] = now.Unix()
	claimsCopy["exp"] = now.Add(expiry).Unix()

	token := jwt.NewWithClaims(method, claimsCopy)
	return token.SignedString(secret)
}

// ParseJWT verifies the provided token string and returns the claims.
func ParseJWT(tokenString string, secret []byte, method jwt.SigningMethod) (jwt.MapClaims, error) {
	claims := jwt.MapClaims{}

	token, err := jwt.ParseWithClaims(tokenString, &claims, func(t *jwt.Token) (any, error) {
		if t.Method.Alg() != method.Alg() {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		return secret, nil
	})
	if err != nil {
		return nil, err
	}

	if !token.Valid {
		return nil, fmt.Errorf("invalid token")
	}

	return claims, nil
}

// bearerTokenFromRequest extracts the Bearer token from the Authorization header
func bearerTokenFromRequest(r *http.Request) (string, error) {
	ah := r.Header["Authorization"]
	if len(ah) == 0 {
		return "", fmt.Errorf("missing authorization header")
	}

	if len(ah[0]) > 7 && ah[0][:7] == "Bearer " {
		return ah[0][7:], nil
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
		user.Authorities = make([]string, len(roles))
		for i, r := range roles {
			if s, ok := r.(string); ok {
				user.Authorities[i] = s
			}
		}
	} else if roles, ok := claims["authorities"].([]any); ok {
		user.Authorities = make([]string, len(roles))
		for i, r := range roles {
			if s, ok := r.(string); ok {
				user.Authorities[i] = s
			}
		}
	} else if scope, ok := claims["scope"].(string); ok && scope != "" {
		user.Authorities = strings.Split(scope, " ")
	}

	return user
}
