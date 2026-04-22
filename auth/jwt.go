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
	authHeader := r.Header.Get("Authorization")
	if authHeader == "" {
		return User{}, errors.New("missing authorization header")
	}

	tokenString := strings.TrimPrefix(authHeader, "Bearer ")
	if tokenString == authHeader {
		return User{}, errors.New("invalid authorization header")
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
		if j.ClaimsFunc != nil {
			return j.ClaimsFunc(claims), nil
		}
		return DefaultUserMapper(claims), nil
	}

	return User{}, errors.New("invalid token")
}

// GenerateJWT creates a JWT token with the given claims.
func GenerateJWT(secret []byte, method jwt.SigningMethod, claims jwt.MapClaims, expiry time.Duration) (string, error) {
	now := time.Now()
	claims["iat"] = now.Unix()
	claims["exp"] = now.Add(expiry).Unix()

	token := jwt.NewWithClaims(method, claims)
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
