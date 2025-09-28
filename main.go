package main

import (
	"log"
	"net/http"
	"os"

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

	if err := database.SeedAdminUser(); err != nil {
		log.Fatalf("Could not seed admin user: %v", err)
	}
	database.ActivateTestUser()

	handlers.WsHub = handlers.NewHub()
	go handlers.WsHub.Run()

	fs := http.FileServer(http.Dir("./static"))
	http.Handle("/static/", http.StripPrefix("/static/", fs))

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
	if err := http.ListenAndServe(":"+port, nil); err != nil {
		log.Fatal(err)
	}
}


