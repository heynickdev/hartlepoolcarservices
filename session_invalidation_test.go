package main

import (
	"context"
	"fmt"
	"hcs-full/database"
	"hcs-full/database/db"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"golang.org/x/crypto/bcrypt"
)

func createTestUserForSession(t *testing.T, email string) *db.User {
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte("testpassword123"), bcrypt.DefaultCost)
	if err != nil {
		t.Fatalf("Failed to hash password: %v", err)
	}

	params := db.CreateUserParams{
		Name:                     "Test User",
		Email:                    email,
		PasswordHash:             string(hashedPassword),
		Phone:                    "1234567890",
		IsAdmin:                  false,
		EmailVerificationToken:   pgtype.Text{Valid: false},
		EmailVerificationExpires: pgtype.Timestamptz{Valid: false},
	}

	user, err := database.Queries.CreateUser(context.Background(), params)
	if err != nil {
		t.Fatalf("Failed to create test user: %v", err)
	}

	// Verify the user's email
	err = database.Queries.VerifyUserEmail(context.Background(), user.ID)
	if err != nil {
		t.Fatalf("Failed to verify test user email: %v", err)
	}

	return &user
}

func cleanupTestUserForSession(t *testing.T, email string) {
	err := database.Queries.DeleteUserByEmail(context.Background(), email)
	if err != nil {
		t.Logf("Failed to cleanup test user: %v", err)
	}
}

func TestTokenBlacklisting(t *testing.T) {
	setupTestDB(t)

	testEmail := fmt.Sprintf("test-blacklist-%d@example.com", time.Now().Unix())
	user := createTestUserForSession(t, testEmail)
	defer cleanupTestUserForSession(t, testEmail)

	// Test blacklisting a token
	fakeToken := "fake.jwt.token"
	userID, err := uuid.FromBytes(user.ID.Bytes[:])
	if err != nil {
		t.Fatalf("Failed to convert user ID: %v", err)
	}

	// Blacklist the token
	err = database.BlacklistToken(fakeToken, userID, "test_reason")
	if err != nil {
		t.Errorf("Failed to blacklist token: %v", err)
	}

	// Check if token is blacklisted
	isBlacklisted, err := database.IsTokenBlacklisted(fakeToken)
	if err != nil {
		t.Errorf("Failed to check token blacklist: %v", err)
	}

	if !isBlacklisted {
		t.Errorf("Expected token to be blacklisted")
	}
}

func TestBlacklistAllUserTokens(t *testing.T) {
	setupTestDB(t)

	testEmail := fmt.Sprintf("test-blacklist-all-%d@example.com", time.Now().Unix())
	user := createTestUserForSession(t, testEmail)
	defer cleanupTestUserForSession(t, testEmail)

	userID, err := uuid.FromBytes(user.ID.Bytes[:])
	if err != nil {
		t.Fatalf("Failed to convert user ID: %v", err)
	}

	// Blacklist all user tokens
	err = database.BlacklistAllUserTokens(userID, "admin_change")
	if err != nil {
		t.Errorf("Failed to blacklist all user tokens: %v", err)
	}
}

func TestPasswordChangeValidation(t *testing.T) {
	// Test the password validation logic
	currentPasswordHash, _ := bcrypt.GenerateFromPassword([]byte("currentpassword"), bcrypt.DefaultCost)

	tests := []struct {
		name        string
		currentPwd  string
		newPwd      string
		expectError bool
		errorMsg    string
	}{
		{
			name:        "Same password",
			currentPwd:  "currentpassword",
			newPwd:      "currentpassword",
			expectError: true,
			errorMsg:    "should reject same password",
		},
		{
			name:        "Different password",
			currentPwd:  "currentpassword",
			newPwd:      "newpassword123",
			expectError: false,
			errorMsg:    "",
		},
		{
			name:        "Short password",
			currentPwd:  "currentpassword",
			newPwd:      "short",
			expectError: true,
			errorMsg:    "should reject short password",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Validate current password
			err := bcrypt.CompareHashAndPassword(currentPasswordHash, []byte(tt.currentPwd))
			if err != nil {
				t.Errorf("Current password validation failed: %v", err)
				return
			}

			// Check if new password is same as current
			err = bcrypt.CompareHashAndPassword(currentPasswordHash, []byte(tt.newPwd))
			samePassword := (err == nil)

			// Check password length
			shortPassword := len(tt.newPwd) < 8

			hasError := samePassword || shortPassword

			if hasError != tt.expectError {
				if tt.expectError {
					t.Errorf("Expected error for %s, but got none", tt.errorMsg)
				} else {
					t.Errorf("Expected no error, but got validation failure")
				}
			}
		})
	}
}

func TestSessionInvalidationCommands(t *testing.T) {
	setupTestDB(t)

	testEmail := fmt.Sprintf("test-session-cmd-%d@example.com", time.Now().Unix())
	user := createTestUserForSession(t, testEmail)
	defer cleanupTestUserForSession(t, testEmail)

	// Test making user admin (should invalidate sessions)
	err := database.MakeAdmin(user.Email)
	if err != nil {
		t.Errorf("Failed to make user admin: %v", err)
	}

	// Verify user is now admin
	updatedUser, err := database.Queries.GetUserByEmail(context.Background(), user.Email)
	if err != nil {
		t.Errorf("Failed to get updated user: %v", err)
	}
	if !updatedUser.IsAdmin {
		t.Errorf("User should be admin after makeAdmin command")
	}

	// Test removing admin privileges (should invalidate sessions)
	err = database.RemoveAdmin(user.Email)
	if err != nil {
		t.Errorf("Failed to remove admin privileges: %v", err)
	}

	// Verify user is no longer admin
	updatedUser, err = database.Queries.GetUserByEmail(context.Background(), user.Email)
	if err != nil {
		t.Errorf("Failed to get updated user: %v", err)
	}
	if updatedUser.IsAdmin {
		t.Errorf("User should not be admin after removeAdmin command")
	}
}
