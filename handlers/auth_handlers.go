package handlers

import (
	"context"
	"hcs-full/database"
	"hcs-full/database/db"
	"hcs-full/models"
	"hcs-full/utils"
	"log"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5"
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
			Name:                        name,
			Email:                       email,
			PasswordHash:                hashedPassword,
			Phone:                       phone,
			IsAdmin:                     false,
			EmailVerificationToken:      pgtype.Text{String: verificationToken, Valid: true},
			EmailVerificationExpires:    pgtype.Timestamptz{Time: expiresAt, Valid: true},
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
	data := models.PageData{Title: "Login"}
	if success == "true" {
		data.Success = "Registration successful! Please log in."
	} else if signup == "true" {
		data.Success = "Registration successful! Please check your email and click the verification link before logging in."
	} else if verified == "true" {
		data.Success = "Email verified successfully! You can now log in."
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

	// Update email
	err = database.Queries.UpdateUserEmail(context.Background(), db.UpdateUserEmailParams{
		ID:    pgtype.UUID{Bytes: claims.UserID, Valid: true},
		Email: newEmail,
	})
	if err != nil {
		log.Printf("Error updating email: %v", err)
		http.Redirect(w, r, "/profile?error=Error+updating+email", http.StatusSeeOther)
		return
	}

	http.Redirect(w, r, "/profile?success=Email+updated+successfully", http.StatusSeeOther)
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


