package database

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"hcs-full/database/db"
	"hcs-full/models"
	"hcs-full/utils"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
)

// HashToken creates a SHA256 hash of a JWT token for blacklisting
func HashToken(token string) string {
	hash := sha256.Sum256([]byte(token))
	return hex.EncodeToString(hash[:])
}

// BlacklistToken adds a token to the blacklist
func BlacklistToken(token string, userID uuid.UUID, reason string) error {
	tokenHash := HashToken(token)

	// Parse the token to get expiration time
	claims, err := utils.ParseJWT(token)
	if err != nil {
		// If we can't parse the token, set a reasonable expiration time
		claims = &models.Claims{
			RegisteredClaims: jwt.RegisteredClaims{
				ExpiresAt: jwt.NewNumericDate(time.Now().Add(24 * time.Hour)),
			},
		}
	}

	return Queries.BlacklistToken(context.Background(), db.BlacklistTokenParams{
		TokenHash: tokenHash,
		UserID:    pgtype.UUID{Bytes: userID, Valid: true},
		Reason:    reason,
		ExpiresAt: pgtype.Timestamptz{Time: claims.RegisteredClaims.ExpiresAt.Time, Valid: true},
	})
}

// BlacklistAllUserTokens invalidates all tokens for a specific user
func BlacklistAllUserTokens(userID uuid.UUID, reason string) error {
	// Set expiration to cover current and future tokens
	expiresAt := time.Now().Add(24 * time.Hour)

	return Queries.BlacklistAllUserTokens(context.Background(), db.BlacklistAllUserTokensParams{
		UserID:    pgtype.UUID{Bytes: userID, Valid: true},
		Reason:    reason,
		ExpiresAt: pgtype.Timestamptz{Time: expiresAt, Valid: true},
	})
}

// IsTokenBlacklisted checks if a token is in the blacklist
func IsTokenBlacklisted(token string) (bool, error) {
	tokenHash := HashToken(token)
	return Queries.IsTokenBlacklisted(context.Background(), tokenHash)
}

// ValidateJWTWithBlacklist parses and validates a JWT token, checking blacklist
func ValidateJWTWithBlacklist(tokenStr string) (*models.Claims, error) {
	// First check if token is blacklisted
	blacklisted, err := IsTokenBlacklisted(tokenStr)
	if err != nil {
		return nil, err
	}
	if blacklisted {
		return nil, jwt.ErrTokenInvalidClaims
	}

	// Then validate the token normally
	return utils.ParseJWT(tokenStr)
}
