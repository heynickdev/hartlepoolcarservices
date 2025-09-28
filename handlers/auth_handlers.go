package handlers

import (
	"context"
	"fmt"
	"hcs-full/database"
	"hcs-full/database/db"
	"hcs-full/models"
	"hcs-full/utils"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"golang.org/x/crypto/bcrypt"
)

func SignupHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodPost {
		name := utils.SanitizeInput(r.FormValue("name"))
		email := utils.SanitizeInput(r.FormValue("email"))
		password := r.FormValue("password")
		confirmPassword := r.FormValue("confirm_password")
		phone := utils.SanitizeInput(r.FormValue("phone"))

		// Check if passwords match
		if password != confirmPassword {
			RenderTemplate(w, r, "signup.html", models.PageData{Title: "Sign Up", Error: "Passwords do not match."})
			return
		}

		if len(password) < 8 {
			RenderTemplate(w, r, "signup.html", models.PageData{Title: "Sign Up", Error: "Password must be at least 8 characters long."})
			return
		}

		hashedPassword, err := utils.HashPassword(password)
		if err != nil {
			http.Error(w, "Server error, unable to hash password.", http.StatusInternalServerError)
			return
		}

		// Generate verification token
		verificationToken, err := utils.GenerateVerificationToken()
		if err != nil {
			log.Printf("Could not generate verification token: %v", err)
			http.Error(w, "Server error, unable to generate verification token.", http.StatusInternalServerError)
			return
		}

		// Set token expiry to 24 hours from now
		expiresAt := time.Now().Add(24 * time.Hour)

		params := db.CreateUserParams{
			Name:                     name,
			Email:                    email,
			PasswordHash:             hashedPassword,
			Phone:                    phone,
			IsAdmin:                  false,
			EmailVerificationToken:   pgtype.Text{String: verificationToken, Valid: true},
			EmailVerificationExpires: pgtype.Timestamptz{Time: expiresAt, Valid: true},
		}

		user, err := database.Queries.CreateUser(context.Background(), params)
		if err != nil {
			log.Printf("Could not create user: %v", err)
			RenderTemplate(w, r, "signup.html", models.PageData{Title: "Sign Up", Error: "Email already exists."})
			return
		}

		// Send verification email
		emailService := utils.NewEmailService()
		err = emailService.SendVerificationEmail(user.Email, verificationToken)
		if err != nil {
			log.Printf("Failed to send verification email: %v", err)
			// Still redirect to success page, but log the error
		}

		http.Redirect(w, r, "/login?signup=true", http.StatusSeeOther)
		return
	}

	RenderTemplate(w, r, "signup.html", models.PageData{Title: "Sign Up"})
}

func LoginHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodPost {
		email := utils.SanitizeInput(r.FormValue("email"))
		password := r.FormValue("password")

		user, err := database.Queries.GetUserByEmail(context.Background(), email)
		if err != nil {
			if err == pgx.ErrNoRows {
				RenderTemplate(w, r, "login.html", models.PageData{Title: "Login", Error: "Invalid email or password."})
			} else {
				http.Error(w, "Database error", http.StatusInternalServerError)
			}
			return
		}

		err = bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password))
		if err != nil {
			RenderTemplate(w, r, "login.html", models.PageData{Title: "Login", Error: "Invalid email or password."})
			return
		}

		// Check if email is verified
		if !user.EmailVerified {
			RenderTemplate(w, r, "login.html", models.PageData{Title: "Login", Error: "Please verify your email address before logging in. Check your inbox for a verification email."})
			return
		}

		token, err := utils.GenerateJWT(user.ID.Bytes, user.Email, user.IsAdmin)
		if err != nil {
			http.Error(w, "Could not generate token", http.StatusInternalServerError)
			return
		}

		http.SetCookie(w, &http.Cookie{
			Name:     "token",
			Value:    token,
			Expires:  time.Now().Add(24 * time.Hour),
			HttpOnly: true,
			Path:     "/",
			SameSite: http.SameSiteStrictMode,
		})

		if user.IsAdmin {
			http.Redirect(w, r, "/admin/dashboard", http.StatusSeeOther)
		} else {
			http.Redirect(w, r, "/dashboard", http.StatusSeeOther)
		}
		return
	}

	success := r.URL.Query().Get("success")
	signup := r.URL.Query().Get("signup")
	verified := r.URL.Query().Get("verified")
	error := r.URL.Query().Get("error")

	data := models.PageData{Title: "Login"}

	// Handle general success messages from URL
	if success != "" && success != "true" {
		data.Success = success
	} else if success == "true" {
		data.Success = "Registration successful! Please log in."
	} else if signup == "true" {
		data.Success = "Registration successful! Please check your email and click the verification link before logging in."
	} else if verified == "true" {
		data.Success = "Email verified successfully! You can now log in."
	}

	// Handle general error messages from URL
	if error != "" {
		data.Error = error
	}
	RenderTemplate(w, r, "login.html", data)
}

func LogoutHandler(w http.ResponseWriter, r *http.Request) {
	http.SetCookie(w, &http.Cookie{
		Name:     "token",
		Value:    "",
		Expires:  time.Now().Add(-1 * time.Hour), // Set expiry to the past
		HttpOnly: true,
		Path:     "/",
	})
	http.Redirect(w, r, "/login", http.StatusSeeOther)
}

func ProfileHandler(w http.ResponseWriter, r *http.Request) {
	claims, ok := r.Context().Value("userClaims").(*models.Claims)
	if !ok {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	user, err := database.Queries.GetUserByID(context.Background(), pgtype.UUID{Bytes: claims.UserID, Valid: true})
	if err != nil {
		log.Printf("Error fetching user: %v", err)
		http.Error(w, "Error loading user data", http.StatusInternalServerError)
		return
	}

	data := models.PageData{
		Title:           "Profile Settings",
		IsAuthenticated: true,
		User:            &user,
		Success:         r.URL.Query().Get("success"),
		Error:           r.URL.Query().Get("error"),
	}

	RenderTemplate(w, r, "profile.html", data)
}

func UpdateEmailHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Redirect(w, r, "/profile", http.StatusSeeOther)
		return
	}

	claims, ok := r.Context().Value("userClaims").(*models.Claims)
	if !ok {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	newEmail := utils.SanitizeInput(r.FormValue("new_email"))
	currentPassword := r.FormValue("current_password")

	if newEmail == "" || currentPassword == "" {
		http.Redirect(w, r, "/profile?error=All+fields+are+required", http.StatusSeeOther)
		return
	}

	// Get current user to verify password
	user, err := database.Queries.GetUserByID(context.Background(), pgtype.UUID{Bytes: claims.UserID, Valid: true})
	if err != nil {
		log.Printf("Error fetching user: %v", err)
		http.Redirect(w, r, "/profile?error=Error+loading+user+data", http.StatusSeeOther)
		return
	}

	// Verify current password
	err = bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(currentPassword))
	if err != nil {
		http.Redirect(w, r, "/profile?error=Current+password+is+incorrect", http.StatusSeeOther)
		return
	}

	// Check if new email already exists
	_, err = database.Queries.GetUserByEmail(context.Background(), newEmail)
	if err == nil {
		// Email exists, so we can't use it
		http.Redirect(w, r, "/profile?error=Email+already+exists", http.StatusSeeOther)
		return
	} else if err != pgx.ErrNoRows {
		// Some other database error occurred
		log.Printf("Error checking email: %v", err)
		http.Redirect(w, r, "/profile?error=Database+error", http.StatusSeeOther)
		return
	}
	// err == sql.ErrNoRows means email doesn't exist, which is what we want

	// Generate email change token
	changeToken := uuid.New().String()
	changeExpires := time.Now().Add(24 * time.Hour) // Token expires in 24 hours

	// Store email change request
	err = database.Queries.SetEmailChangeRequest(context.Background(), db.SetEmailChangeRequestParams{
		ID:                 pgtype.UUID{Bytes: claims.UserID, Valid: true},
		PendingEmail:       pgtype.Text{String: newEmail, Valid: true},
		EmailChangeToken:   pgtype.Text{String: changeToken, Valid: true},
		EmailChangeExpires: pgtype.Timestamptz{Time: changeExpires, Valid: true},
	})
	if err != nil {
		log.Printf("Error setting email change request: %v", err)
		http.Redirect(w, r, "/profile?error=Error+processing+email+change+request", http.StatusSeeOther)
		return
	}

	// Send verification email to new email address
	baseURL := os.Getenv("BASE_URL")
	if baseURL == "" {
		baseURL = "http://localhost:8080"
	}

	// Ensure HTTPS for production domain
	if baseURL == "hartlepoolcarservices.com" {
		baseURL = "https://hartlepoolcarservices.com"
	}

	verifyURL := fmt.Sprintf("%s/verify-email-change?token=%s", baseURL, changeToken)
	subject := "Verify Your New Email Address - Hartlepool Car Services"
	body := fmt.Sprintf(`
	<html>
	<body style="font-family: Arial, sans-serif; line-height: 1.6; color: #333;">
		<div style="max-width: 600px; margin: 0 auto; padding: 20px;">
			<h2 style="color: #2c3e50;">Email Address Change Verification</h2>
			<p>Hello %s,</p>
			<p>You requested to change your email address to this one (%s) on your Hartlepool Car Services account. To confirm this email change, please click the button below:</p>

			<div style="text-align: center; margin: 30px 0;">
				<a href="%s" style="background-color: #3498db; color: white; padding: 12px 30px; text-decoration: none; border-radius: 5px; display: inline-block;">Verify New Email Address</a>
			</div>

			<p>If the button doesn't work, you can also copy and paste this link into your browser:</p>
			<p style="word-break: break-all; color: #7f8c8d;">%s</p>

			<p style="margin-top: 30px;">This verification link will expire in 24 hours.</p>

			<hr style="border: none; border-top: 1px solid #eee; margin: 30px 0;">
			<p style="font-size: 12px; color: #7f8c8d;">
				If you didn't request this email change, please ignore this email and contact our support team.
			</p>
			<p style="font-size: 12px; color: #7f8c8d;">
				Hartlepool Car Services<br>
				Email: info@hartlepoolcarservices.com
			</p>
		</div>
	</body>
	</html>
	`, user.Name, newEmail, verifyURL, verifyURL)

	emailService := utils.NewEmailService()
	err = emailService.SendEmail(newEmail, subject, body)
	if err != nil {
		log.Printf("Error sending email change verification: %v", err)
		http.Redirect(w, r, "/profile?error=Error+sending+verification+email", http.StatusSeeOther)
		return
	}

	http.Redirect(w, r, "/profile?success=Verification+email+sent+to+your+new+email+address.+Please+check+your+inbox+and+click+the+link+to+confirm+the+change.", http.StatusSeeOther)
}

func UpdatePasswordHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Redirect(w, r, "/profile", http.StatusSeeOther)
		return
	}

	claims, ok := r.Context().Value("userClaims").(*models.Claims)
	if !ok {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	currentPassword := r.FormValue("current_password")
	newPassword := r.FormValue("new_password")
	confirmPassword := r.FormValue("confirm_password")

	if currentPassword == "" || newPassword == "" || confirmPassword == "" {
		http.Redirect(w, r, "/profile?error=All+fields+are+required", http.StatusSeeOther)
		return
	}

	if newPassword != confirmPassword {
		http.Redirect(w, r, "/profile?error=New+passwords+do+not+match", http.StatusSeeOther)
		return
	}

	if len(newPassword) < 8 {
		http.Redirect(w, r, "/profile?error=Password+must+be+at+least+8+characters+long", http.StatusSeeOther)
		return
	}

	// Get current user to verify password
	user, err := database.Queries.GetUserByID(context.Background(), pgtype.UUID{Bytes: claims.UserID, Valid: true})
	if err != nil {
		log.Printf("Error fetching user: %v", err)
		http.Redirect(w, r, "/profile?error=Error+loading+user+data", http.StatusSeeOther)
		return
	}

	// Verify current password
	err = bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(currentPassword))
	if err != nil {
		http.Redirect(w, r, "/profile?error=Current+password+is+incorrect", http.StatusSeeOther)
		return
	}

	// Check if new password is the same as current password
	err = bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(newPassword))
	if err == nil {
		http.Redirect(w, r, "/profile?error=New+password+cannot+be+the+same+as+your+current+password", http.StatusSeeOther)
		return
	}

	// Hash new password
	hashedPassword, err := utils.HashPassword(newPassword)
	if err != nil {
		log.Printf("Error hashing password: %v", err)
		http.Redirect(w, r, "/profile?error=Error+updating+password", http.StatusSeeOther)
		return
	}

	// Update password
	err = database.Queries.UpdateUserPassword(context.Background(), db.UpdateUserPasswordParams{
		ID:           pgtype.UUID{Bytes: claims.UserID, Valid: true},
		PasswordHash: hashedPassword,
	})
	if err != nil {
		log.Printf("Error updating password: %v", err)
		http.Redirect(w, r, "/profile?error=Error+updating+password", http.StatusSeeOther)
		return
	}

	http.Redirect(w, r, "/profile?success=Password+updated+successfully", http.StatusSeeOther)
}

func VerifyEmailHandler(w http.ResponseWriter, r *http.Request) {
	token := r.URL.Query().Get("token")
	if token == "" {
		http.Redirect(w, r, "/login?error=Invalid+verification+link", http.StatusSeeOther)
		return
	}

	// Find user by verification token
	user, err := database.Queries.GetUserByVerificationToken(context.Background(), pgtype.Text{String: token, Valid: true})
	if err != nil {
		if err == pgx.ErrNoRows {
			http.Redirect(w, r, "/login?error=Invalid+or+expired+verification+link", http.StatusSeeOther)
		} else {
			log.Printf("Error finding user by verification token: %v", err)
			http.Error(w, "Database error", http.StatusInternalServerError)
		}
		return
	}

	// Verify the email
	err = database.Queries.VerifyUserEmail(context.Background(), user.ID)
	if err != nil {
		log.Printf("Error verifying user email: %v", err)
		http.Error(w, "Database error", http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/login?verified=true", http.StatusSeeOther)
}

func VerifyEmailReminderHandler(w http.ResponseWriter, r *http.Request) {
	RenderTemplate(w, r, "verify_email.html", models.PageData{Title: "Email Verification Required"})
}

func ForgotPasswordHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet {
		data := models.PageData{
			Title: "Forgot Password",
		}
		RenderTemplate(w, r, "forgot_password.html", data)
		return
	}

	if r.Method == http.MethodPost {
		email := r.FormValue("email")
		if email == "" {
			data := models.PageData{
				Title: "Forgot Password",
				Error: "Email is required",
			}
			RenderTemplate(w, r, "forgot_password.html", data)
			return
		}

		// Check if user exists
		user, err := database.Queries.GetUserByEmail(context.Background(), email)
		if err != nil {
			// Don't reveal if email exists or not for security
			data := models.PageData{
				Title:   "Forgot Password",
				Success: "If an account with that email exists, we've sent you a password reset link.",
			}
			RenderTemplate(w, r, "forgot_password.html", data)
			return
		}

		// Check if there's already an active reset token
		activeToken, err := database.Queries.GetActivePasswordResetToken(context.Background(), email)
		var resetToken string
		var resetExpires time.Time

		if err == nil {
			// Active token exists, reuse it
			resetToken = activeToken.PasswordResetToken.String
			resetExpires = activeToken.PasswordResetExpires.Time
		} else {
			// No active token, generate a new one
			resetToken = uuid.New().String()
			resetExpires = time.Now().Add(1 * time.Hour) // Token expires in 1 hour

			// Store reset token in database
			err = database.Queries.SetPasswordResetToken(context.Background(), db.SetPasswordResetTokenParams{
				Email:                email,
				PasswordResetToken:   pgtype.Text{String: resetToken, Valid: true},
				PasswordResetExpires: pgtype.Timestamptz{Time: resetExpires, Valid: true},
			})
			if err != nil {
				log.Printf("Error setting password reset token: %v", err)
				data := models.PageData{
					Title: "Forgot Password",
					Error: "Error processing request. Please try again.",
				}
				RenderTemplate(w, r, "forgot_password.html", data)
				return
			}
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
		subject := "Password Reset - Hartlepool Car Services"
		body := fmt.Sprintf(`
		<html>
		<body style="font-family: Arial, sans-serif; line-height: 1.6; color: #333;">
			<div style="max-width: 600px; margin: 0 auto; padding: 20px;">
				<h2 style="color: #2c3e50;">Password Reset Request</h2>
				<p>Hello %s,</p>
				<p>You requested a password reset for your Hartlepool Car Services account. To reset your password, please click the button below:</p>

				<div style="text-align: center; margin: 30px 0;">
					<a href="%s" style="background-color: #3498db; color: white; padding: 12px 30px; text-decoration: none; border-radius: 5px; display: inline-block;">Reset Password</a>
				</div>

				<p>If the button doesn't work, you can also copy and paste this link into your browser:</p>
				<p style="word-break: break-all; color: #7f8c8d;">%s</p>

				<p style="margin-top: 30px;">This password reset link will expire in 1 hour.</p>

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
		`, user.Name, resetURL, resetURL)

		emailService := utils.NewEmailService()
		err = emailService.SendEmail(user.Email, subject, body)
		if err != nil {
			log.Printf("Error sending password reset email: %v", err)
			data := models.PageData{
				Title: "Forgot Password",
				Error: "Error sending email. Please try again.",
			}
			RenderTemplate(w, r, "forgot_password.html", data)
			return
		}

		// Redirect to login page with success message
		http.Redirect(w, r, "/login?success=Password+reset+link+sent!+Check+your+email+for+instructions.", http.StatusSeeOther)
		return
	}
}

func ResetPasswordHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet {
		token := r.URL.Query().Get("token")
		if token == "" {
			data := models.PageData{
				Title: "Reset Password",
				Error: "Invalid reset link",
			}
			RenderTemplate(w, r, "reset_password.html", data)
			return
		}

		// Verify token
		_, err := database.Queries.GetUserByPasswordResetToken(context.Background(), pgtype.Text{String: token, Valid: true})
		if err != nil {
			data := models.PageData{
				Title: "Reset Password",
				Error: "Invalid or expired reset link",
			}
			RenderTemplate(w, r, "reset_password.html", data)
			return
		}

		data := models.PageData{
			Title: "Reset Password",
			Token: token,
		}
		RenderTemplate(w, r, "reset_password.html", data)
		return
	}

	if r.Method == http.MethodPost {
		token := r.FormValue("token")
		password := r.FormValue("password")
		confirmPassword := r.FormValue("confirm_password")

		if token == "" || password == "" || confirmPassword == "" {
			data := models.PageData{
				Title: "Reset Password",
				Token: token,
				Error: "All fields are required",
			}
			RenderTemplate(w, r, "reset_password.html", data)
			return
		}

		if password != confirmPassword {
			data := models.PageData{
				Title: "Reset Password",
				Token: token,
				Error: "Passwords do not match",
			}
			RenderTemplate(w, r, "reset_password.html", data)
			return
		}

		if len(password) < 8 {
			data := models.PageData{
				Title: "Reset Password",
				Token: token,
				Error: "Password must be at least 8 characters long",
			}
			RenderTemplate(w, r, "reset_password.html", data)
			return
		}

		// Verify token and get user
		user, err := database.Queries.GetUserByPasswordResetToken(context.Background(), pgtype.Text{String: token, Valid: true})
		if err != nil {
			data := models.PageData{
				Title: "Reset Password",
				Token: token,
				Error: "Invalid or expired reset link",
			}
			RenderTemplate(w, r, "reset_password.html", data)
			return
		}

		// Hash new password
		hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
		if err != nil {
			log.Printf("Error hashing password: %v", err)
			data := models.PageData{
				Title: "Reset Password",
				Token: token,
				Error: "Error processing request. Please try again.",
			}
			RenderTemplate(w, r, "reset_password.html", data)
			return
		}

		// Update password and clear reset token
		err = database.Queries.ResetPassword(context.Background(), db.ResetPasswordParams{
			ID:           user.ID,
			PasswordHash: string(hashedPassword),
		})
		if err != nil {
			log.Printf("Error resetting password: %v", err)
			data := models.PageData{
				Title: "Reset Password",
				Token: token,
				Error: "Error updating password. Please try again.",
			}
			RenderTemplate(w, r, "reset_password.html", data)
			return
		}

		// Redirect to login page with success message
		http.Redirect(w, r, "/login?success=Password+reset+successfully!+You+can+now+login+with+your+new+password.", http.StatusSeeOther)
		return
	}
}

func ResendResetHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet {
		data := models.PageData{
			Title: "Resend Reset Link",
		}
		RenderTemplate(w, r, "resend_reset.html", data)
		return
	}

	if r.Method == http.MethodPost {
		email := r.FormValue("email")
		if email == "" {
			data := models.PageData{
				Title: "Resend Reset Link",
				Error: "Email is required",
			}
			RenderTemplate(w, r, "resend_reset.html", data)
			return
		}

		// Check if user exists and has an active reset token
		user, err := database.Queries.GetUserByEmail(context.Background(), email)
		if err != nil {
			// Don't reveal if email exists or not for security
			data := models.PageData{
				Title:   "Resend Reset Link",
				Success: "If an account with that email has an active reset request, we've resent the link.",
			}
			RenderTemplate(w, r, "resend_reset.html", data)
			return
		}

		// Check if there's an active reset token
		activeToken, err := database.Queries.GetActivePasswordResetToken(context.Background(), email)
		if err != nil {
			// No active token
			data := models.PageData{
				Title: "Resend Reset Link",
				Error: "No active password reset request found. Please request a new password reset.",
			}
			RenderTemplate(w, r, "resend_reset.html", data)
			return
		}

		// Send reset email with existing token
		baseURL := os.Getenv("BASE_URL")
		if baseURL == "" {
			baseURL = "http://localhost:8080"
		}

		// Ensure HTTPS for production domain
		if baseURL == "hartlepoolcarservices.com" {
			baseURL = "https://hartlepoolcarservices.com"
		}

		resetURL := fmt.Sprintf("%s/reset-password?token=%s", baseURL, activeToken.PasswordResetToken.String)
		subject := "Password Reset - Hartlepool Car Services (Resent)"
		body := fmt.Sprintf(`
		<html>
		<body style="font-family: Arial, sans-serif; line-height: 1.6; color: #333;">
			<div style="max-width: 600px; margin: 0 auto; padding: 20px;">
				<h2 style="color: #2c3e50;">Password Reset Request (Resent)</h2>
				<p>Hello %s,</p>
				<p>You requested to resend your password reset link for your Hartlepool Car Services account. To reset your password, please click the button below:</p>

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
		`, user.Name, resetURL, resetURL, activeToken.PasswordResetExpires.Time.Format("January 2, 2006 at 3:04 PM"))

		emailService := utils.NewEmailService()
		err = emailService.SendEmail(user.Email, subject, body)
		if err != nil {
			log.Printf("Error sending password reset email: %v", err)
			data := models.PageData{
				Title: "Resend Reset Link",
				Error: "Error sending email. Please try again.",
			}
			RenderTemplate(w, r, "resend_reset.html", data)
			return
		}

		// Show success message
		data := models.PageData{
			Title:   "Resend Reset Link",
			Success: "Password reset link has been resent! Check your email for instructions.",
		}
		RenderTemplate(w, r, "resend_reset.html", data)
		return
	}
}

func VerifyEmailChangeHandler(w http.ResponseWriter, r *http.Request) {
	token := r.URL.Query().Get("token")
	if token == "" {
		http.Redirect(w, r, "/profile?error=Invalid+verification+link", http.StatusSeeOther)
		return
	}

	// Verify token and get user
	user, err := database.Queries.GetUserByEmailChangeToken(context.Background(), pgtype.Text{String: token, Valid: true})
	if err != nil {
		http.Redirect(w, r, "/profile?error=Invalid+or+expired+verification+link", http.StatusSeeOther)
		return
	}

	// Confirm email change
	err = database.Queries.ConfirmEmailChange(context.Background(), user.ID)
	if err != nil {
		log.Printf("Error confirming email change: %v", err)
		http.Redirect(w, r, "/profile?error=Error+updating+email", http.StatusSeeOther)
		return
	}

	http.Redirect(w, r, "/profile?success=Email+address+updated+successfully", http.StatusSeeOther)
}
