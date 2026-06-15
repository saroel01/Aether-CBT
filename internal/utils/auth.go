package utils

import (
	"crypto/rand"
	"encoding/hex"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
)

var jwtSecret = []byte("")

// SetJWTSecret allows overriding the secret at startup (from config)
func SetJWTSecret(secret string) {
	if secret == "" {
		panic("FATAL: JWT secret tidak boleh kosong. Set JWT_SECRET di environment sebelum menjalankan aplikasi.")
	}
	jwtSecret = []byte(secret)
}

func GenerateSecureToken(byteLength int) (string, error) {
	if byteLength <= 0 {
		byteLength = 32
	}
	bytes := make([]byte, byteLength)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes), nil
}

// GetJWTSecret returns the current JWT secret (for middleware that can't import config)
func GetJWTSecret() string {
	if len(jwtSecret) == 0 {
		panic("FATAL: JWT secret belum di-set. Panggil SetJWTSecret terlebih dahulu dengan nilai dari environment.")
	}
	return string(jwtSecret)
}

// HashPassword hashes a password using bcrypt
func HashPassword(password string) (string, error) {
	bytes, err := bcrypt.GenerateFromPassword([]byte(password), 14)
	return string(bytes), err
}

// CheckPasswordHash compares a plaintext password against a stored bcrypt hash using
// bcrypt's constant-time comparison. A stored value that is not a bcrypt hash (empty,
// too short, or plaintext) is always rejected — there is no plaintext fallback. The
// short-prefix guard avoids feeding garbage into CompareHashAndPassword.
func CheckPasswordHash(password, hash string) bool {
	if len(hash) < 7 || (hash[:4] != "$2a$" && hash[:4] != "$2b$" && hash[:4] != "$2y$") {
		return false
	}
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)) == nil
}

// GenerateToken generates a JWT token
func GenerateToken(userID int, tenantID int, role string) (string, error) {
	claims := jwt.MapClaims{
		"user_id":   userID,
		"tenant_id": tenantID,
		"role":      role,
		"exp":       time.Now().Add(time.Hour * 24).Unix(),
		"iat":       time.Now().Unix(),
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(jwtSecret)
}

// ValidateToken validates a JWT token
func ValidateToken(tokenString string) (*jwt.Token, error) {
	return jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, jwt.ErrSignatureInvalid
		}
		return jwtSecret, nil
	})
}
