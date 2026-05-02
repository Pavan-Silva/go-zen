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
	Secret        []byte
	SigningMethod jwt.SigningMethod
	ClaimsFunc   func(claims jwt.MapClaims) User
}

func (j *JWTAuth) Authenticate(r *http.Request) (User, error) {
	if j == nil {
		return User{}, errors.New("jwt auth is not configured")
	}
	if j.SigningMethod == nil {
		return User{}, errors.New("jwt signing method is not configured")
	}
	if len(j.Secret) == 0 {
		return User{}, errors.New("jwt secret is not configured")
	}

	tokenString, err := bearerTokenFromRequest(r)
	if err != nil {
		return User{}, err
	}

	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (any, error) {
		if token.Method.Alg() != j.SigningMethod.Alg() {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return j.Secret, nil
	})

	if err != nil {
		return User{}, err
	}

	if claims, ok := token.Claims.(jwt.MapClaims); ok && token.Valid {
		return userFromClaims(j.ClaimsFunc, claims), nil
	}

	return User{}, errors.New("invalid token")
}

// GenerateJWT creates a JWT token with the given claims.
// It makes a copy of the claims map before adding iat and exp to avoid
// surprising side effects on the caller's map.
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
		// Enforce expected signing method
		if t.Method.Alg() != method.Alg() {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		return secret, nil
	})
	if err != nil {
		return nil, err
	}

	// Validate token and claims
	if !token.Valid {
		return nil, fmt.Errorf("invalid token")
	}

	return claims, nil
}

// bearerTokenFromRequest extracts the Bearer token from the Authorization header.
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

// userFromClaims maps jwt.MapClaims to User, either via a custom ClaimsFunc
// or the DefaultUserMapper.
func userFromClaims(claimsFunc func(jwt.MapClaims) User, claims jwt.MapClaims) User {
	if claimsFunc != nil {
		return claimsFunc(claims)
	}
	return DefaultUserMapper(claims)
}

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
