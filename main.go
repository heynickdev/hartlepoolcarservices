package main

import (
	"bufio"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"

	"hcs-full/database"
	"hcs-full/handlers"
	"hcs-full/middleware"

	"github.com/joho/godotenv"
)

func main() {
	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found, using environment variables")
	}

	database.ConnectDB()
	defer database.CloseDB()

	if err := database.RunMigrations(); err != nil {
		log.Fatalf("Could not run database migrations: %v", err)
	}

	handlers.WsHub = handlers.NewHub()
	go handlers.WsHub.Run()

	// Start command-line interface
	go startCommandInterface()

	fs := http.FileServer(http.Dir("./static"))
	http.Handle("/static/", http.StripPrefix("/static/", fs))

	// Serve robots.txt from static directory
	http.HandleFunc("/robots.txt", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		http.ServeFile(w, r, "./static/robots.txt")
	})

	// Serve sitemap.xml from static directory
	http.HandleFunc("/sitemap.xml", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/xml")
		http.ServeFile(w, r, "./static/sitemap.xml")
	})

	// Public routes
	http.HandleFunc("/", handlers.HomePage)
	http.HandleFunc("/about", handlers.AboutPage)
	http.HandleFunc("/services", handlers.ServicesPage)
	http.HandleFunc("/contact", handlers.ContactPage)
	http.HandleFunc("/privacy-policy", handlers.PrivacyPolicyPage)
	http.HandleFunc("/terms-and-conditions", handlers.TermsAndConditionsPage)

	// Auth routes
	http.HandleFunc("/login", handlers.LoginHandler)
	http.HandleFunc("/signup", handlers.SignupHandler)
	http.HandleFunc("/logout", handlers.LogoutHandler)
	http.HandleFunc("/verify-email", handlers.VerifyEmailHandler)
	http.HandleFunc("/verify-email-reminder", handlers.VerifyEmailReminderHandler)
	http.HandleFunc("/verify-email-change", handlers.VerifyEmailChangeHandler)
	http.HandleFunc("/forgot-password", handlers.ForgotPasswordHandler)
	http.HandleFunc("/resend-reset", handlers.ResendResetHandler)
	http.HandleFunc("/reset-password", handlers.ResetPasswordHandler)

	// Authenticated routes
	http.Handle("/dashboard", middleware.AuthMiddleware(http.HandlerFunc(handlers.DashboardHandler)))
	http.Handle("/vehicle", middleware.AuthMiddleware(http.HandlerFunc(handlers.VehicleDetailHandler)))
	http.Handle("/create-appointment", middleware.AuthMiddleware(http.HandlerFunc(handlers.CreateAppointmentHandler)))
	http.Handle("/cancel-appointment", middleware.AuthMiddleware(http.HandlerFunc(handlers.UserCancelAppointmentHandler)))

	// Profile routes
	http.Handle("/profile", middleware.AuthMiddleware(http.HandlerFunc(handlers.ProfileHandler)))
	http.Handle("/update-email", middleware.AuthMiddleware(http.HandlerFunc(handlers.UpdateEmailHandler)))
	http.Handle("/update-password", middleware.AuthMiddleware(http.HandlerFunc(handlers.UpdatePasswordHandler)))

	// Car management routes
	http.Handle("/add-car", middleware.AuthMiddleware(http.HandlerFunc(handlers.AddCarHandler)))
	http.Handle("/delete-car", middleware.AuthMiddleware(http.HandlerFunc(handlers.DeleteCarHandler)))
	http.Handle("/refresh-car", middleware.AuthMiddleware(http.HandlerFunc(handlers.RefreshCarDataHandler)))

	// WebSocket route
	http.Handle("/ws", middleware.SoftAuthMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handlers.ServeWs(handlers.WsHub, w, r)
	})))

	// Super Admin routes
	superAdminRoutes := http.NewServeMux()
	superAdminRoutes.Handle("/super-admin/dashboard", http.HandlerFunc(handlers.SuperAdminDashboardHandler))
	superAdminRoutes.Handle("/super-admin/users", http.HandlerFunc(handlers.SuperAdminUsersHandler))
	superAdminRoutes.Handle("/super-admin/vehicles", http.HandlerFunc(handlers.SuperAdminVehicleManagementHandler))
	superAdminRoutes.Handle("/super-admin/update-user-role", http.HandlerFunc(handlers.SuperAdminUpdateUserRoleHandler))
	superAdminRoutes.Handle("/super-admin/delete-user", http.HandlerFunc(handlers.SuperAdminDeleteUserHandler))
	superAdminRoutes.Handle("/super-admin/activate-user", http.HandlerFunc(handlers.SuperAdminActivateUserHandler))
	superAdminRoutes.Handle("/super-admin/send-password-reset", http.HandlerFunc(handlers.SuperAdminSendPasswordResetHandler))
	superAdminRoutes.Handle("/super-admin/allocate-vehicle", http.HandlerFunc(handlers.SuperAdminAllocateVehicleHandler))
	superAdminRoutes.Handle("/super-admin/remove-vehicle", http.HandlerFunc(handlers.SuperAdminRemoveVehicleHandler))
	superAdminRoutes.Handle("/super-admin/toggle-dashboard", http.HandlerFunc(handlers.SuperAdminToggleDashboardHandler))
	superAdminRoutes.Handle("/super-admin/view-user", http.HandlerFunc(handlers.SuperAdminViewUserHandler))
	superAdminRoutes.Handle("/super-admin/add-car-for-user", http.HandlerFunc(handlers.SuperAdminAddCarForUserHandler))
	superAdminRoutes.Handle("/super-admin/delete-car-for-user", http.HandlerFunc(handlers.SuperAdminDeleteCarForUserHandler))
	superAdminRoutes.Handle("/api/super-admin/stats", http.HandlerFunc(handlers.SuperAdminSystemStatsHandler))
	http.Handle("/super-admin/", middleware.AuthMiddleware(middleware.SuperAdminMiddleware(superAdminRoutes)))
	http.Handle("/api/super-admin/", middleware.AuthMiddleware(middleware.SuperAdminMiddleware(superAdminRoutes)))

	// Admin routes
	adminRoutes := http.NewServeMux()
	adminRoutes.Handle("/admin/overview", http.HandlerFunc(handlers.AdminOverviewHandler))
	adminRoutes.Handle("/admin/dashboard", http.HandlerFunc(handlers.AdminDashboardHandler))
	adminRoutes.Handle("/admin/update-status", http.HandlerFunc(handlers.AdminUpdateAppointmentStatusHandler))
	adminRoutes.Handle("/admin/delete-appointment", http.HandlerFunc(handlers.AdminDeleteAppointmentHandler))
	adminRoutes.Handle("/admin/cars", http.HandlerFunc(handlers.AdminCarsHandler))
	adminRoutes.Handle("/admin/vehicle", http.HandlerFunc(handlers.AdminVehicleDetailHandler))
	adminRoutes.Handle("/admin/delete-car", http.HandlerFunc(handlers.AdminDeleteCarHandler))
	adminRoutes.Handle("/api/admin/calendar", http.HandlerFunc(handlers.AdminCalendarHandler))
	http.Handle("/admin/", middleware.AuthMiddleware(middleware.AdminMiddleware(adminRoutes)))
	http.Handle("/api/admin/", middleware.AuthMiddleware(middleware.AdminMiddleware(adminRoutes)))

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	log.Printf("Server starting on :%s...", port)

	// Wrap the default mux with security and cache middlewares
	handler := middleware.SecurityHeadersMiddleware(middleware.CacheHeadersMiddleware(http.DefaultServeMux))

	if err := http.ListenAndServe(":"+port, handler); err != nil {
		log.Fatal(err)
	}
}

// startCommandInterface starts a command-line interface for user management
func startCommandInterface() {
	scanner := bufio.NewScanner(os.Stdin)

	fmt.Println("\n=== HCS User Management CLI ===")
	fmt.Println("Server is running. Type 'help' for available commands.")
	fmt.Print("hcs> ")

	for scanner.Scan() {
		input := strings.TrimSpace(scanner.Text())
		if input == "" {
			fmt.Print("hcs> ")
			continue
		}

		if input == "exit" || input == "quit" {
			fmt.Println("Goodbye!")
			os.Exit(0)
		}

		err := database.ProcessCommand(input)
		if err != nil {
			fmt.Printf("Error: %v\n", err)
		}

		fmt.Print("hcs> ")
	}

	if err := scanner.Err(); err != nil {
		log.Printf("Error reading from stdin: %v", err)
	}
}
