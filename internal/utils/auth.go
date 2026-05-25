package utils

import (
	"crypto/rand"
	"encoding/hex"
	"strings"
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

// CheckPasswordHash compares password with hash
func CheckPasswordHash(password, hash string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
	return err == nil
}

func CheckPasswordOrPlaintext(password, stored string) bool {
	if strings.HasPrefix(stored, "$2a$") || strings.HasPrefix(stored, "$2b$") || strings.HasPrefix(stored, "$2y$") {
		return CheckPasswordHash(password, stored)
	}
	return password == stored
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
