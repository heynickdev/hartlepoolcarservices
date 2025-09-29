package utils

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"hcs-full/models"
	"os"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

var jwtKey = []byte(os.Getenv("JWT_SECRET"))

func HashPassword(password string) (string, error) {
	bytes, err := bcrypt.GenerateFromPassword([]byte(password), 14)
	return string(bytes), err
}

func GenerateJWT(userID uuid.UUID, email string, role string) (string, error) {
	expirationTime := time.Now().Add(24 * time.Hour)
	claims := &models.Claims{
		UserID:  userID,
		Email:   email,
		Role:    role,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(expirationTime),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(jwtKey)
}

func ParseJWT(tokenStr string) (*models.Claims, error) {
	claims := &models.Claims{}
	token, err := jwt.ParseWithClaims(tokenStr, claims, func(token *jwt.Token) (interface{}, error) {
		return jwtKey, nil
	})

	if err != nil {
		return nil, err
	}

	if !token.Valid {
		return nil, err
	}

	return claims, nil
}

func GenerateVerificationToken() (string, error) {
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes), nil
}

// HashToken creates a SHA256 hash of a JWT token for blacklisting
func HashToken(token string) string {
	hash := sha256.Sum256([]byte(token))
	return hex.EncodeToString(hash[:])
}

// ValidateJWT parses and validates a JWT token, checking blacklist
func ValidateJWT(tokenStr string) (*models.Claims, error) {
	// Parse the token first to get basic validation
	claims, err := ParseJWT(tokenStr)
	if err != nil {
		return nil, err
	}

	// The blacklist check will be done in the middleware
	// We avoid the import cycle by not importing database here
	return claims, nil
}

// Role constants
const (
	RoleUser       = "user"
	RoleAdmin      = "admin"
	RoleSuperAdmin = "super_admin"
)

// IsAdmin checks if the user has admin or super admin role
func IsAdmin(role string) bool {
	return role == RoleAdmin || role == RoleSuperAdmin
}

// IsSuperAdmin checks if the user has super admin role
func IsSuperAdmin(role string) bool {
	return role == RoleSuperAdmin
}

// CanAccessAdmin checks if the user can access admin features
func CanAccessAdmin(role string) bool {
	return IsAdmin(role)
}

// CanAccessSuperAdmin checks if the user can access super admin features
func CanAccessSuperAdmin(role string) bool {
	return IsSuperAdmin(role)
}
