package auth

import (
	"golang.org/x/crypto/bcrypt"
)

// ValidatePassword securely validates a plaintext password against a stored bcrypt hash.
// Uses constant-time comparison via bcrypt.CompareHashAndPassword.
func ValidatePassword(storedHash, plain string) bool {
	if storedHash == "" || plain == "" {
		return false
	}

	return bcrypt.CompareHashAndPassword([]byte(storedHash), []byte(plain)) == nil
}

// HashPassword hashes a password using bcrypt.
// The salt is generated internally by bcrypt. If cost is not provided,
// the default bcrypt cost is used.
func HashPassword(password string, costs ...int) (string, error) {
	cost := bcrypt.DefaultCost
	if len(costs) > 0 && costs[0] > 0 {
		cost = costs[0]
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(password), cost)
	return string(hash), err
}
