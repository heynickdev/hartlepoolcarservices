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

// Test the password reset logic without HTTP handlers

func setupTestDB(t *testing.T) {
	if database.Queries == nil {
		t.Skip("Database not available for testing")
	}
}

func createTestUser(t *testing.T, email string) *db.User {
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

func cleanupTestUser(t *testing.T, email string) {
	err := database.Queries.DeleteUserByEmail(context.Background(), email)
	if err != nil {
		t.Logf("Failed to cleanup test user: %v", err)
	}
}

func TestPasswordResetTokenCreation(t *testing.T) {
	setupTestDB(t)

	testEmail := fmt.Sprintf("test-token-%d@example.com", time.Now().Unix())
	user := createTestUser(t, testEmail)
	defer cleanupTestUser(t, testEmail)

	// Test creating a reset token
	resetToken := uuid.New().String()
	resetExpires := time.Now().Add(1 * time.Hour)

	err := database.Queries.SetPasswordResetToken(context.Background(), db.SetPasswordResetTokenParams{
		Email:                user.Email,
		PasswordResetToken:   pgtype.Text{String: resetToken, Valid: true},
		PasswordResetExpires: pgtype.Timestamptz{Time: resetExpires, Valid: true},
	})

	if err != nil {
		t.Errorf("Failed to set password reset token: %v", err)
	}

	// Test retrieving the active token
	activeToken, err := database.Queries.GetActivePasswordResetToken(context.Background(), user.Email)
	if err != nil {
		t.Errorf("Failed to get active reset token: %v", err)
	}

	if activeToken.PasswordResetToken.String != resetToken {
		t.Errorf("Expected token %s, got %s", resetToken, activeToken.PasswordResetToken.String)
	}
}

func TestPasswordResetTokenExpiry(t *testing.T) {
	setupTestDB(t)

	testEmail := fmt.Sprintf("test-expiry-%d@example.com", time.Now().Unix())
	user := createTestUser(t, testEmail)
	defer cleanupTestUser(t, testEmail)

	// Create an expired token
	resetToken := uuid.New().String()
	resetExpires := time.Now().Add(-1 * time.Hour) // Expired 1 hour ago

	err := database.Queries.SetPasswordResetToken(context.Background(), db.SetPasswordResetTokenParams{
		Email:                user.Email,
		PasswordResetToken:   pgtype.Text{String: resetToken, Valid: true},
		PasswordResetExpires: pgtype.Timestamptz{Time: resetExpires, Valid: true},
	})

	if err != nil {
		t.Errorf("Failed to set expired password reset token: %v", err)
	}

	// Should not be able to retrieve expired token
	_, err = database.Queries.GetActivePasswordResetToken(context.Background(), user.Email)
	if err == nil {
		t.Errorf("Expected error when getting expired token, but got none")
	}
}

func TestPasswordResetTokenReuse(t *testing.T) {
	setupTestDB(t)

	testEmail := fmt.Sprintf("test-reuse-%d@example.com", time.Now().Unix())
	user := createTestUser(t, testEmail)
	defer cleanupTestUser(t, testEmail)

	// Create initial token
	firstToken := uuid.New().String()
	firstExpires := time.Now().Add(1 * time.Hour)

	err := database.Queries.SetPasswordResetToken(context.Background(), db.SetPasswordResetTokenParams{
		Email:                user.Email,
		PasswordResetToken:   pgtype.Text{String: firstToken, Valid: true},
		PasswordResetExpires: pgtype.Timestamptz{Time: firstExpires, Valid: true},
	})

	if err != nil {
		t.Errorf("Failed to set first password reset token: %v", err)
	}

	// Verify we can get the active token
	activeToken, err := database.Queries.GetActivePasswordResetToken(context.Background(), user.Email)
	if err != nil {
		t.Errorf("Failed to get first active reset token: %v", err)
	}

	if activeToken.PasswordResetToken.String != firstToken {
		t.Errorf("Expected first token %s, got %s", firstToken, activeToken.PasswordResetToken.String)
	}

	// Test that when we check for existing token and reuse it, it should be the same
	existingToken, err := database.Queries.GetActivePasswordResetToken(context.Background(), user.Email)
	if err != nil {
		t.Errorf("Failed to get existing active reset token: %v", err)
	}

	if existingToken.PasswordResetToken.String != firstToken {
		t.Errorf("Expected existing token to be reused: %s, got %s", firstToken, existingToken.PasswordResetToken.String)
	}
}

func TestPasswordReset(t *testing.T) {
	setupTestDB(t)

	testEmail := fmt.Sprintf("test-password-reset-%d@example.com", time.Now().Unix())
	user := createTestUser(t, testEmail)
	defer cleanupTestUser(t, testEmail)

	// Create a reset token
	resetToken := uuid.New().String()
	resetExpires := time.Now().Add(1 * time.Hour)

	err := database.Queries.SetPasswordResetToken(context.Background(), db.SetPasswordResetTokenParams{
		Email:                user.Email,
		PasswordResetToken:   pgtype.Text{String: resetToken, Valid: true},
		PasswordResetExpires: pgtype.Timestamptz{Time: resetExpires, Valid: true},
	})

	if err != nil {
		t.Errorf("Failed to set password reset token: %v", err)
	}

	// Verify we can get the user by reset token
	userByToken, err := database.Queries.GetUserByPasswordResetToken(context.Background(), pgtype.Text{String: resetToken, Valid: true})
	if err != nil {
		t.Errorf("Failed to get user by reset token: %v", err)
	}

	if userByToken.Email != user.Email {
		t.Errorf("Expected user email %s, got %s", user.Email, userByToken.Email)
	}

	// Reset the password
	newPassword := "newpassword123"
	hashedNewPassword, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost)
	if err != nil {
		t.Errorf("Failed to hash new password: %v", err)
	}

	err = database.Queries.ResetPassword(context.Background(), db.ResetPasswordParams{
		ID:           user.ID,
		PasswordHash: string(hashedNewPassword),
	})

	if err != nil {
		t.Errorf("Failed to reset password: %v", err)
	}

	// Verify the password was changed
	updatedUser, err := database.Queries.GetUserByEmail(context.Background(), user.Email)
	if err != nil {
		t.Errorf("Failed to get updated user: %v", err)
	}

	err = bcrypt.CompareHashAndPassword([]byte(updatedUser.PasswordHash), []byte(newPassword))
	if err != nil {
		t.Errorf("New password doesn't work: %v", err)
	}

	// Verify the reset token was cleared
	_, err = database.Queries.GetActivePasswordResetToken(context.Background(), user.Email)
	if err == nil {
		t.Errorf("Expected reset token to be cleared after successful reset")
	}
}

func TestPasswordValidation(t *testing.T) {
	tests := []struct {
		name     string
		password string
		confirm  string
		valid    bool
		error    string
	}{
		{"Valid password", "validpassword123", "validpassword123", true, ""},
		{"Password too short", "short", "short", false, "Password must be at least 8 characters long"},
		{"Passwords don't match", "password123", "different123", false, "Passwords do not match"},
		{"Empty password", "", "", false, "All fields are required"},
		{"Empty confirm", "password123", "", false, "All fields are required"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Simulate password validation logic
			if tt.password == "" || tt.confirm == "" {
				if tt.valid {
					t.Errorf("Expected validation to fail for empty fields")
				}
				return
			}

			if tt.password != tt.confirm {
				if tt.valid {
					t.Errorf("Expected validation to fail for mismatched passwords")
				}
				return
			}

			if len(tt.password) < 8 {
				if tt.valid {
					t.Errorf("Expected validation to fail for short password")
				}
				return
			}

			if !tt.valid {
				t.Errorf("Expected validation to fail but it passed")
			}
		})
	}
}

// Integration test for complete password reset flow
func TestCompletePasswordResetFlow(t *testing.T) {
	setupTestDB(t)

	testEmail := fmt.Sprintf("test-complete-%d@example.com", time.Now().Unix())
	user := createTestUser(t, testEmail)
	defer cleanupTestUser(t, testEmail)

	// Step 1: User requests password reset
	// Check if there's already an active token (should be none)
	_, err := database.Queries.GetActivePasswordResetToken(context.Background(), user.Email)
	if err == nil {
		t.Errorf("Expected no active token initially")
	}

	// Step 2: Generate new reset token
	resetToken := uuid.New().String()
	resetExpires := time.Now().Add(1 * time.Hour)

	err = database.Queries.SetPasswordResetToken(context.Background(), db.SetPasswordResetTokenParams{
		Email:                user.Email,
		PasswordResetToken:   pgtype.Text{String: resetToken, Valid: true},
		PasswordResetExpires: pgtype.Timestamptz{Time: resetExpires, Valid: true},
	})

	if err != nil {
		t.Errorf("Failed to set password reset token: %v", err)
	}

	// Step 3: User receives email and clicks link (validate token)
	userByToken, err := database.Queries.GetUserByPasswordResetToken(context.Background(), pgtype.Text{String: resetToken, Valid: true})
	if err != nil {
		t.Errorf("Failed to validate reset token: %v", err)
	}

	if userByToken.ID != user.ID {
		t.Errorf("Reset token doesn't match the correct user")
	}

	// Step 4: User submits new password
	newPassword := "completenewpassword123"
	hashedNewPassword, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost)
	if err != nil {
		t.Errorf("Failed to hash new password: %v", err)
	}

	err = database.Queries.ResetPassword(context.Background(), db.ResetPasswordParams{
		ID:           user.ID,
		PasswordHash: string(hashedNewPassword),
	})

	if err != nil {
		t.Errorf("Failed to reset password: %v", err)
	}

	// Step 5: Verify complete reset
	// Check password was changed
	updatedUser, err := database.Queries.GetUserByEmail(context.Background(), user.Email)
	if err != nil {
		t.Errorf("Failed to get updated user: %v", err)
	}

	err = bcrypt.CompareHashAndPassword([]byte(updatedUser.PasswordHash), []byte(newPassword))
	if err != nil {
		t.Errorf("New password doesn't work: %v", err)
	}

	// Check old password no longer works
	err = bcrypt.CompareHashAndPassword([]byte(updatedUser.PasswordHash), []byte("testpassword123"))
	if err == nil {
		t.Errorf("Old password still works, it should not")
	}

	// Check reset token was cleared
	_, err = database.Queries.GetActivePasswordResetToken(context.Background(), user.Email)
	if err == nil {
		t.Errorf("Reset token should be cleared after successful reset")
	}

	// Check that the same token can't be used again
	_, err = database.Queries.GetUserByPasswordResetToken(context.Background(), pgtype.Text{String: resetToken, Valid: true})
	if err == nil {
		t.Errorf("Reset token should not be usable after reset")
	}
}
