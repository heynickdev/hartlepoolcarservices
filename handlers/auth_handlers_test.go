package handlers

import (
	"context"
	"fmt"
	"hcs-full/database"
	"hcs-full/database/db"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"golang.org/x/crypto/bcrypt"
)

// Test helper functions
func setupTestDB(t *testing.T) {
	// This would normally set up a test database
	// For now, we'll assume the database is already set up
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

// Test ForgotPasswordHandler
func TestForgotPasswordHandler_GET(t *testing.T) {
	setupTestDB(t)

	req, err := http.NewRequest("GET", "/forgot-password", nil)
	if err != nil {
		t.Fatal(err)
	}

	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(ForgotPasswordHandler)

	handler.ServeHTTP(rr, req)

	if status := rr.Code; status != http.StatusOK {
		t.Errorf("handler returned wrong status code: got %v want %v",
			status, http.StatusOK)
	}

	if !strings.Contains(rr.Body.String(), "Reset your password") {
		t.Errorf("Expected forgot password page content not found")
	}
}

func TestForgotPasswordHandler_POST_ValidEmail(t *testing.T) {
	setupTestDB(t)

	testEmail := fmt.Sprintf("test-forgot-%d@example.com", time.Now().Unix())
	user := createTestUser(t, testEmail)
	defer cleanupTestUser(t, testEmail)

	form := url.Values{}
	form.Add("email", user.Email)

	req, err := http.NewRequest("POST", "/forgot-password", strings.NewReader(form.Encode()))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Add("Content-Type", "application/x-www-form-urlencoded")

	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(ForgotPasswordHandler)

	handler.ServeHTTP(rr, req)

	// Should redirect to login with success message
	if status := rr.Code; status != http.StatusSeeOther {
		t.Errorf("handler returned wrong status code: got %v want %v",
			status, http.StatusSeeOther)
	}

	location := rr.Header().Get("Location")
	if !strings.Contains(location, "/login") || !strings.Contains(location, "success=") {
		t.Errorf("Expected redirect to login with success message, got: %s", location)
	}

	// Verify reset token was created
	activeToken, err := database.Queries.GetActivePasswordResetToken(context.Background(), user.Email)
	if err != nil {
		t.Errorf("Expected active reset token to be created: %v", err)
	} else if activeToken.PasswordResetToken.String == "" {
		t.Errorf("Expected non-empty reset token")
	}
}

func TestForgotPasswordHandler_POST_InvalidEmail(t *testing.T) {
	setupTestDB(t)

	form := url.Values{}
	form.Add("email", "nonexistent@example.com")

	req, err := http.NewRequest("POST", "/forgot-password", strings.NewReader(form.Encode()))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Add("Content-Type", "application/x-www-form-urlencoded")

	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(ForgotPasswordHandler)

	handler.ServeHTTP(rr, req)

	// Should still show success message for security
	if !strings.Contains(rr.Body.String(), "If an account with that email exists") {
		t.Errorf("Expected security message for non-existent email")
	}
}

func TestForgotPasswordHandler_POST_ExistingActiveToken(t *testing.T) {
	setupTestDB(t)

	testEmail := fmt.Sprintf("test-existing-%d@example.com", time.Now().Unix())
	user := createTestUser(t, testEmail)
	defer cleanupTestUser(t, testEmail)

	// Create an existing reset token
	resetToken := uuid.New().String()
	resetExpires := time.Now().Add(1 * time.Hour)
	err := database.Queries.SetPasswordResetToken(context.Background(), db.SetPasswordResetTokenParams{
		Email:                user.Email,
		PasswordResetToken:   pgtype.Text{String: resetToken, Valid: true},
		PasswordResetExpires: pgtype.Timestamptz{Time: resetExpires, Valid: true},
	})
	if err != nil {
		t.Fatalf("Failed to set initial reset token: %v", err)
	}

	// Request another reset
	form := url.Values{}
	form.Add("email", user.Email)

	req, err := http.NewRequest("POST", "/forgot-password", strings.NewReader(form.Encode()))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Add("Content-Type", "application/x-www-form-urlencoded")

	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(ForgotPasswordHandler)

	handler.ServeHTTP(rr, req)

	// Should still redirect successfully
	if status := rr.Code; status != http.StatusSeeOther {
		t.Errorf("handler returned wrong status code: got %v want %v",
			status, http.StatusSeeOther)
	}

	// Verify the same token is reused
	activeToken, err := database.Queries.GetActivePasswordResetToken(context.Background(), user.Email)
	if err != nil {
		t.Errorf("Expected active reset token: %v", err)
	} else if activeToken.PasswordResetToken.String != resetToken {
		t.Errorf("Expected token to be reused, got different token")
	}
}

// Test ResendResetHandler
func TestResendResetHandler_GET(t *testing.T) {
	setupTestDB(t)

	req, err := http.NewRequest("GET", "/resend-reset", nil)
	if err != nil {
		t.Fatal(err)
	}

	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(ResendResetHandler)

	handler.ServeHTTP(rr, req)

	if status := rr.Code; status != http.StatusOK {
		t.Errorf("handler returned wrong status code: got %v want %v",
			status, http.StatusOK)
	}

	if !strings.Contains(rr.Body.String(), "Resend Reset Link") {
		t.Errorf("Expected resend reset page content not found")
	}
}

func TestResendResetHandler_POST_WithActiveToken(t *testing.T) {
	setupTestDB(t)

	testEmail := fmt.Sprintf("test-resend-%d@example.com", time.Now().Unix())
	user := createTestUser(t, testEmail)
	defer cleanupTestUser(t, testEmail)

	// Create an active reset token
	resetToken := uuid.New().String()
	resetExpires := time.Now().Add(1 * time.Hour)
	err := database.Queries.SetPasswordResetToken(context.Background(), db.SetPasswordResetTokenParams{
		Email:                user.Email,
		PasswordResetToken:   pgtype.Text{String: resetToken, Valid: true},
		PasswordResetExpires: pgtype.Timestamptz{Time: resetExpires, Valid: true},
	})
	if err != nil {
		t.Fatalf("Failed to set reset token: %v", err)
	}

	form := url.Values{}
	form.Add("email", user.Email)

	req, err := http.NewRequest("POST", "/resend-reset", strings.NewReader(form.Encode()))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Add("Content-Type", "application/x-www-form-urlencoded")

	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(ResendResetHandler)

	handler.ServeHTTP(rr, req)

	if status := rr.Code; status != http.StatusOK {
		t.Errorf("handler returned wrong status code: got %v want %v",
			status, http.StatusOK)
	}

	if !strings.Contains(rr.Body.String(), "Password reset link has been resent") {
		t.Errorf("Expected success message for resend")
	}
}

func TestResendResetHandler_POST_NoActiveToken(t *testing.T) {
	setupTestDB(t)

	testEmail := fmt.Sprintf("test-no-token-%d@example.com", time.Now().Unix())
	user := createTestUser(t, testEmail)
	defer cleanupTestUser(t, testEmail)

	form := url.Values{}
	form.Add("email", user.Email)

	req, err := http.NewRequest("POST", "/resend-reset", strings.NewReader(form.Encode()))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Add("Content-Type", "application/x-www-form-urlencoded")

	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(ResendResetHandler)

	handler.ServeHTTP(rr, req)

	if status := rr.Code; status != http.StatusOK {
		t.Errorf("handler returned wrong status code: got %v want %v",
			status, http.StatusOK)
	}

	if !strings.Contains(rr.Body.String(), "No active password reset request found") {
		t.Errorf("Expected error message for no active token")
	}
}

// Test ResetPasswordHandler
func TestResetPasswordHandler_GET_ValidToken(t *testing.T) {
	setupTestDB(t)

	testEmail := fmt.Sprintf("test-reset-%d@example.com", time.Now().Unix())
	user := createTestUser(t, testEmail)
	defer cleanupTestUser(t, testEmail)

	// Create a valid reset token
	resetToken := uuid.New().String()
	resetExpires := time.Now().Add(1 * time.Hour)
	err := database.Queries.SetPasswordResetToken(context.Background(), db.SetPasswordResetTokenParams{
		Email:                user.Email,
		PasswordResetToken:   pgtype.Text{String: resetToken, Valid: true},
		PasswordResetExpires: pgtype.Timestamptz{Time: resetExpires, Valid: true},
	})
	if err != nil {
		t.Fatalf("Failed to set reset token: %v", err)
	}

	req, err := http.NewRequest("GET", "/reset-password?token="+resetToken, nil)
	if err != nil {
		t.Fatal(err)
	}

	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(ResetPasswordHandler)

	handler.ServeHTTP(rr, req)

	if status := rr.Code; status != http.StatusOK {
		t.Errorf("handler returned wrong status code: got %v want %v",
			status, http.StatusOK)
	}

	if !strings.Contains(rr.Body.String(), "Reset Password") {
		t.Errorf("Expected reset password form")
	}
}

func TestResetPasswordHandler_GET_InvalidToken(t *testing.T) {
	setupTestDB(t)

	req, err := http.NewRequest("GET", "/reset-password?token=invalid-token", nil)
	if err != nil {
		t.Fatal(err)
	}

	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(ResetPasswordHandler)

	handler.ServeHTTP(rr, req)

	if status := rr.Code; status != http.StatusOK {
		t.Errorf("handler returned wrong status code: got %v want %v",
			status, http.StatusOK)
	}

	if !strings.Contains(rr.Body.String(), "Invalid or expired reset link") {
		t.Errorf("Expected error message for invalid token")
	}
}

func TestResetPasswordHandler_POST_ValidReset(t *testing.T) {
	setupTestDB(t)

	testEmail := fmt.Sprintf("test-password-reset-%d@example.com", time.Now().Unix())
	user := createTestUser(t, testEmail)
	defer cleanupTestUser(t, testEmail)

	// Create a valid reset token
	resetToken := uuid.New().String()
	resetExpires := time.Now().Add(1 * time.Hour)
	err := database.Queries.SetPasswordResetToken(context.Background(), db.SetPasswordResetTokenParams{
		Email:                user.Email,
		PasswordResetToken:   pgtype.Text{String: resetToken, Valid: true},
		PasswordResetExpires: pgtype.Timestamptz{Time: resetExpires, Valid: true},
	})
	if err != nil {
		t.Fatalf("Failed to set reset token: %v", err)
	}

	form := url.Values{}
	form.Add("token", resetToken)
	form.Add("password", "newpassword123")
	form.Add("confirm_password", "newpassword123")

	req, err := http.NewRequest("POST", "/reset-password", strings.NewReader(form.Encode()))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Add("Content-Type", "application/x-www-form-urlencoded")

	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(ResetPasswordHandler)

	handler.ServeHTTP(rr, req)

	// Should redirect to login with success message
	if status := rr.Code; status != http.StatusSeeOther {
		t.Errorf("handler returned wrong status code: got %v want %v",
			status, http.StatusSeeOther)
	}

	location := rr.Header().Get("Location")
	if !strings.Contains(location, "/login") || !strings.Contains(location, "success=") {
		t.Errorf("Expected redirect to login with success message, got: %s", location)
	}

	// Verify password was changed and token was cleared
	updatedUser, err := database.Queries.GetUserByEmail(context.Background(), user.Email)
	if err != nil {
		t.Fatalf("Failed to get updated user: %v", err)
	}

	// Test new password works
	err = bcrypt.CompareHashAndPassword([]byte(updatedUser.PasswordHash), []byte("newpassword123"))
	if err != nil {
		t.Errorf("New password doesn't work: %v", err)
	}

	// Verify token was cleared
	_, err = database.Queries.GetActivePasswordResetToken(context.Background(), user.Email)
	if err == nil {
		t.Errorf("Expected reset token to be cleared after successful reset")
	}
}

func TestResetPasswordHandler_POST_PasswordMismatch(t *testing.T) {
	setupTestDB(t)

	testEmail := fmt.Sprintf("test-mismatch-%d@example.com", time.Now().Unix())
	user := createTestUser(t, testEmail)
	defer cleanupTestUser(t, testEmail)

	// Create a valid reset token
	resetToken := uuid.New().String()
	resetExpires := time.Now().Add(1 * time.Hour)
	err := database.Queries.SetPasswordResetToken(context.Background(), db.SetPasswordResetTokenParams{
		Email:                user.Email,
		PasswordResetToken:   pgtype.Text{String: resetToken, Valid: true},
		PasswordResetExpires: pgtype.Timestamptz{Time: resetExpires, Valid: true},
	})
	if err != nil {
		t.Fatalf("Failed to set reset token: %v", err)
	}

	form := url.Values{}
	form.Add("token", resetToken)
	form.Add("password", "newpassword123")
	form.Add("confirm_password", "differentpassword")

	req, err := http.NewRequest("POST", "/reset-password", strings.NewReader(form.Encode()))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Add("Content-Type", "application/x-www-form-urlencoded")

	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(ResetPasswordHandler)

	handler.ServeHTTP(rr, req)

	if status := rr.Code; status != http.StatusOK {
		t.Errorf("handler returned wrong status code: got %v want %v",
			status, http.StatusOK)
	}

	if !strings.Contains(rr.Body.String(), "Passwords do not match") {
		t.Errorf("Expected password mismatch error")
	}
}

func TestResetPasswordHandler_POST_ShortPassword(t *testing.T) {
	setupTestDB(t)

	testEmail := fmt.Sprintf("test-short-%d@example.com", time.Now().Unix())
	user := createTestUser(t, testEmail)
	defer cleanupTestUser(t, testEmail)

	// Create a valid reset token
	resetToken := uuid.New().String()
	resetExpires := time.Now().Add(1 * time.Hour)
	err := database.Queries.SetPasswordResetToken(context.Background(), db.SetPasswordResetTokenParams{
		Email:                user.Email,
		PasswordResetToken:   pgtype.Text{String: resetToken, Valid: true},
		PasswordResetExpires: pgtype.Timestamptz{Time: resetExpires, Valid: true},
	})
	if err != nil {
		t.Fatalf("Failed to set reset token: %v", err)
	}

	form := url.Values{}
	form.Add("token", resetToken)
	form.Add("password", "short")
	form.Add("confirm_password", "short")

	req, err := http.NewRequest("POST", "/reset-password", strings.NewReader(form.Encode()))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Add("Content-Type", "application/x-www-form-urlencoded")

	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(ResetPasswordHandler)

	handler.ServeHTTP(rr, req)

	if status := rr.Code; status != http.StatusOK {
		t.Errorf("handler returned wrong status code: got %v want %v",
			status, http.StatusOK)
	}

	if !strings.Contains(rr.Body.String(), "Password must be at least 8 characters long") {
		t.Errorf("Expected password length error")
	}
}

// Benchmark tests
func BenchmarkForgotPasswordHandler(b *testing.B) {
	setupTestDB(nil)

	testEmail := fmt.Sprintf("bench-test-%d@example.com", time.Now().Unix())
	createTestUser(nil, testEmail)
	defer cleanupTestUser(nil, testEmail)

	form := url.Values{}
	form.Add("email", testEmail)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		req, _ := http.NewRequest("POST", "/forgot-password", strings.NewReader(form.Encode()))
		req.Header.Add("Content-Type", "application/x-www-form-urlencoded")

		rr := httptest.NewRecorder()
		handler := http.HandlerFunc(ForgotPasswordHandler)
		handler.ServeHTTP(rr, req)
	}
}
