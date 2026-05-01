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

// HashPassword hashes a password using bcrypt with the default cost.
// The salt is generated internally by bcrypt.
func HashPassword(password string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	return string(hash), err
}
