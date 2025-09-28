package database

import (
	"context"
	"fmt"
	"hcs-full/database/db"
	"hcs-full/utils"
	"os"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
)

// ShowHelp displays available command-line commands
func ShowHelp() {
	fmt.Println("\n=== User Management Commands ===")
	fmt.Println("Available commands while server is running:")
	fmt.Println("")
	fmt.Println("User Management:")
	fmt.Println("  listUsers             - List all users with their details")
	fmt.Println("  makeAdmin <email>     - Grant admin privileges to user")
	fmt.Println("  removeAdmin <email>   - Remove admin privileges from user")
	fmt.Println("  activateUser <email>  - Activate/verify user account")
	fmt.Println("  deleteUser <email>    - Delete user account and all associated data")
	fmt.Println("")
	fmt.Println("Email Management:")
	fmt.Println("  resendPassword <email> - Resend password reset email")
	fmt.Println("  verifyEmail <email>    - Send verification email")
	fmt.Println("")
	fmt.Println("Other:")
	fmt.Println("  help                  - Show this help message")
	fmt.Println("")
	fmt.Println("Usage: Type the command followed by the email address (where required)")
	fmt.Println("Example: makeAdmin user@example.com")
	fmt.Println("")
}

// ListUsers displays all users with their details
func ListUsers() error {
	users, err := Queries.ListAllUsers(context.Background())
	if err != nil {
		return fmt.Errorf("failed to retrieve users: %v", err)
	}

	if len(users) == 0 {
		fmt.Println("No users found.")
		return nil
	}

	fmt.Printf("\n=== User List (%d users) ===\n", len(users))
	for i, user := range users {
		fmt.Printf("%d. Name: %s\n", i+1, user.Name)
		fmt.Printf("   Email: %s\n", user.Email)
		fmt.Printf("   Admin: %t\n", user.IsAdmin)
		fmt.Printf("   Email Verified: %t\n", user.EmailVerified)
		fmt.Printf("   Created: %s\n", user.CreatedAt.Time.Format("2006-01-02 15:04:05"))
		if i < len(users)-1 {
			fmt.Println()
		}
	}
	fmt.Println()
	return nil
}

// MakeAdmin grants admin privileges to a user by email
func MakeAdmin(email string) error {
	if email == "" {
		return fmt.Errorf("email address is required")
	}

	// Check if user exists
	user, err := Queries.GetUserByEmail(context.Background(), email)
	if err != nil {
		return fmt.Errorf("user with email '%s' not found", email)
	}

	// Check if already admin
	if user.IsAdmin {
		fmt.Printf("User '%s' is already an admin\n", email)
		return nil
	}

	// Make user admin
	err = Queries.MakeUserAdmin(context.Background(), email)
	if err != nil {
		return fmt.Errorf("failed to make user admin: %v", err)
	}

	// Invalidate all user sessions to force re-login with new privileges
	userID, err := uuid.FromBytes(user.ID.Bytes[:])
	if err == nil {
		BlacklistAllUserTokens(userID, "admin_granted")
	}

	fmt.Printf("Successfully granted admin privileges to '%s' (user logged out)\n", email)
	return nil
}

// RemoveAdmin removes admin privileges from a user by email
func RemoveAdmin(email string) error {
	if email == "" {
		return fmt.Errorf("email address is required")
	}

	// Check if user exists
	user, err := Queries.GetUserByEmail(context.Background(), email)
	if err != nil {
		return fmt.Errorf("user with email '%s' not found", email)
	}

	// Check if not admin
	if !user.IsAdmin {
		fmt.Printf("User '%s' is not an admin\n", email)
		return nil
	}

	// Remove admin privileges
	err = Queries.RemoveUserAdmin(context.Background(), email)
	if err != nil {
		return fmt.Errorf("failed to remove admin privileges: %v", err)
	}

	// Invalidate all user sessions to force re-login with updated privileges
	userID, err := uuid.FromBytes(user.ID.Bytes[:])
	if err == nil {
		BlacklistAllUserTokens(userID, "admin_removed")
	}

	fmt.Printf("Successfully removed admin privileges from '%s' (user logged out)\n", email)
	return nil
}

// ActivateUser activates/verifies a user account by email
func ActivateUser(email string) error {
	if email == "" {
		return fmt.Errorf("email address is required")
	}

	// Check if user exists
	user, err := Queries.GetUserByEmail(context.Background(), email)
	if err != nil {
		return fmt.Errorf("user with email '%s' not found", email)
	}

	// Check if already activated
	if user.EmailVerified {
		fmt.Printf("User '%s' is already activated\n", email)
		return nil
	}

	// Activate user
	err = Queries.ActivateUserByEmail(context.Background(), email)
	if err != nil {
		return fmt.Errorf("failed to activate user: %v", err)
	}

	fmt.Printf("Successfully activated user '%s'\n", email)
	return nil
}

// DeleteUser deletes a user account and all associated data by email
func DeleteUser(email string) error {
	if email == "" {
		return fmt.Errorf("email address is required")
	}

	// Check if user exists
	user, err := Queries.GetUserByEmail(context.Background(), email)
	if err != nil {
		return fmt.Errorf("user with email '%s' not found", email)
	}

	// Invalidate all user sessions before deletion
	userID, err := uuid.FromBytes(user.ID.Bytes[:])
	if err == nil {
		BlacklistAllUserTokens(userID, "user_deleted")
	}

	// Delete user (this will cascade delete appointments and cars due to foreign key constraints)
	err = Queries.DeleteUserByEmail(context.Background(), email)
	if err != nil {
		return fmt.Errorf("failed to delete user: %v", err)
	}

	fmt.Printf("Successfully deleted user '%s' and all associated data (user logged out)\n", email)
	return nil
}

// ResendPassword sends a password reset email to the specified user
func ResendPassword(email string) error {
	if email == "" {
		return fmt.Errorf("email address is required")
	}

	// Check if user exists
	user, err := Queries.GetUserByEmail(context.Background(), email)
	if err != nil {
		return fmt.Errorf("user with email '%s' not found", email)
	}

	// Check if there's already an active reset token
	activeToken, err := Queries.GetActivePasswordResetToken(context.Background(), email)
	var resetToken string
	var resetExpires time.Time

	if err == nil {
		// Active token exists, reuse it
		resetToken = activeToken.PasswordResetToken.String
		resetExpires = activeToken.PasswordResetExpires.Time
		fmt.Printf("Reusing existing password reset token for '%s'\n", email)
	} else {
		// No active token, generate a new one
		resetToken = uuid.New().String()
		resetExpires = time.Now().Add(1 * time.Hour) // Token expires in 1 hour

		// Store reset token in database
		err = Queries.SetPasswordResetToken(context.Background(), db.SetPasswordResetTokenParams{
			Email:                email,
			PasswordResetToken:   pgtype.Text{String: resetToken, Valid: true},
			PasswordResetExpires: pgtype.Timestamptz{Time: resetExpires, Valid: true},
		})
		if err != nil {
			return fmt.Errorf("failed to create password reset token: %v", err)
		}
		fmt.Printf("Created new password reset token for '%s'\n", email)
	}

	// Send reset email
	baseURL := os.Getenv("BASE_URL")
	if baseURL == "" {
		baseURL = "http://localhost:8080"
	}

	// Ensure HTTPS for production domain
	if baseURL == "hartlepoolcarservices.com" {
		baseURL = "https://hartlepoolcarservices.com"
	}

	resetURL := fmt.Sprintf("%s/reset-password?token=%s", baseURL, resetToken)
	subject := "Password Reset - Hartlepool Car Services (Admin Resend)"
	body := fmt.Sprintf(`
	<html>
	<body style="font-family: Arial, sans-serif; line-height: 1.6; color: #333;">
		<div style="max-width: 600px; margin: 0 auto; padding: 20px;">
			<h2 style="color: #2c3e50;">Password Reset Request (Admin Resend)</h2>
			<p>Hello %s,</p>
			<p>An administrator has resent your password reset link for your Hartlepool Car Services account. To reset your password, please click the button below:</p>

			<div style="text-align: center; margin: 30px 0;">
				<a href="%s" style="background-color: #3498db; color: white; padding: 12px 30px; text-decoration: none; border-radius: 5px; display: inline-block;">Reset Password</a>
			</div>

			<p>If the button doesn't work, you can also copy and paste this link into your browser:</p>
			<p style="word-break: break-all; color: #7f8c8d;">%s</p>

			<p style="margin-top: 30px;">This password reset link will expire at %s.</p>

			<hr style="border: none; border-top: 1px solid #eee; margin: 30px 0;">
			<p style="font-size: 12px; color: #7f8c8d;">
				If you didn't request this password reset, please ignore this email and your password will remain unchanged.
			</p>
			<p style="font-size: 12px; color: #7f8c8d;">
				Hartlepool Car Services<br>
				Email: info@hartlepoolcarservices.com
			</p>
		</div>
	</body>
	</html>
	`, user.Name, resetURL, resetURL, resetExpires.Format("January 2, 2006 at 3:04 PM"))

	emailService := utils.NewEmailService()
	err = emailService.SendEmail(user.Email, subject, body)
	if err != nil {
		return fmt.Errorf("failed to send password reset email: %v", err)
	}

	fmt.Printf("Successfully sent password reset email to '%s'\n", email)
	return nil
}

// VerifyEmail sends a verification email to the specified user
func VerifyEmail(email string) error {
	if email == "" {
		return fmt.Errorf("email address is required")
	}

	// Check if user exists
	user, err := Queries.GetUserByEmail(context.Background(), email)
	if err != nil {
		return fmt.Errorf("user with email '%s' not found", email)
	}

	// Check if already verified
	if user.EmailVerified {
		fmt.Printf("User '%s' is already verified\n", email)
		return nil
	}

	// Generate new verification token
	verificationToken, err := utils.GenerateVerificationToken()
	if err != nil {
		return fmt.Errorf("failed to generate verification token: %v", err)
	}

	// Set token expiry to 24 hours from now
	expiresAt := time.Now().Add(24 * time.Hour)

	// Update user with new verification token
	err = Queries.SetEmailVerificationToken(context.Background(), db.SetEmailVerificationTokenParams{
		ID:                       user.ID,
		EmailVerificationToken:   pgtype.Text{String: verificationToken, Valid: true},
		EmailVerificationExpires: pgtype.Timestamptz{Time: expiresAt, Valid: true},
	})
	if err != nil {
		return fmt.Errorf("failed to set verification token: %v", err)
	}

	// Send verification email
	baseURL := os.Getenv("BASE_URL")
	if baseURL == "" {
		baseURL = "http://localhost:8080"
	}

	// Ensure HTTPS for production domain
	if baseURL == "hartlepoolcarservices.com" {
		baseURL = "https://hartlepoolcarservices.com"
	}

	verifyURL := fmt.Sprintf("%s/verify-email?token=%s", baseURL, verificationToken)
	subject := "Email Verification - Hartlepool Car Services (Admin Resend)"
	body := fmt.Sprintf(`
	<html>
	<body style="font-family: Arial, sans-serif; line-height: 1.6; color: #333;">
		<div style="max-width: 600px; margin: 0 auto; padding: 20px;">
			<h2 style="color: #2c3e50;">Email Verification (Admin Resend)</h2>
			<p>Hello %s,</p>
			<p>An administrator has resent your email verification link for your Hartlepool Car Services account. To verify your email address and activate your account, please click the button below:</p>

			<div style="text-align: center; margin: 30px 0;">
				<a href="%s" style="background-color: #27ae60; color: white; padding: 12px 30px; text-decoration: none; border-radius: 5px; display: inline-block;">Verify Email Address</a>
			</div>

			<p>If the button doesn't work, you can also copy and paste this link into your browser:</p>
			<p style="word-break: break-all; color: #7f8c8d;">%s</p>

			<p style="margin-top: 30px;">This verification link will expire at %s.</p>

			<hr style="border: none; border-top: 1px solid #eee; margin: 30px 0;">
			<p style="font-size: 12px; color: #7f8c8d;">
				If you didn't create an account with us, please ignore this email.
			</p>
			<p style="font-size: 12px; color: #7f8c8d;">
				Hartlepool Car Services<br>
				Email: info@hartlepoolcarservices.com
			</p>
		</div>
	</body>
	</html>
	`, user.Name, verifyURL, verifyURL, expiresAt.Format("January 2, 2006 at 3:04 PM"))

	emailService := utils.NewEmailService()
	err = emailService.SendEmail(user.Email, subject, body)
	if err != nil {
		return fmt.Errorf("failed to send verification email: %v", err)
	}

	fmt.Printf("Successfully sent verification email to '%s'\n", email)
	return nil
}

// ProcessCommand processes a command input and executes the appropriate function
func ProcessCommand(input string) error {
	input = strings.TrimSpace(input)
	if input == "" {
		return fmt.Errorf("no command provided")
	}

	parts := strings.Fields(input)
	command := strings.ToLower(parts[0])

	switch command {
	case "help":
		ShowHelp()
		return nil
	case "list":
		return ListUsers()
	case "makeadmin":
		if len(parts) < 2 {
			return fmt.Errorf("makeAdmin requires an email address")
		}
		return MakeAdmin(parts[1])
	case "removeadmin":
		if len(parts) < 2 {
			return fmt.Errorf("removeAdmin requires an email address")
		}
		return RemoveAdmin(parts[1])
	case "activateuser":
		if len(parts) < 2 {
			return fmt.Errorf("activateUser requires an email address")
		}
		return ActivateUser(parts[1])
	case "deleteuser":
		if len(parts) < 2 {
			return fmt.Errorf("deleteUser requires an email address")
		}
		return DeleteUser(parts[1])
	case "resendpassword":
		if len(parts) < 2 {
			return fmt.Errorf("resendPassword requires an email address")
		}
		return ResendPassword(parts[1])
	case "verifyemail":
		if len(parts) < 2 {
			return fmt.Errorf("verifyEmail requires an email address")
		}
		return VerifyEmail(parts[1])
	default:
		return fmt.Errorf("unknown command '%s'. Type 'help' for available commands", command)
	}
}
