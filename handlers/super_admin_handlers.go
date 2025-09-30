package handlers

import (
	"context"
	"encoding/json"
	"hcs-full/database"
	"hcs-full/database/db"
	"hcs-full/models"
	"hcs-full/utils"
	"log"
	"net/http"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
)

// SuperAdminDashboardHandler handles the super admin dashboard with overview of all users, roles, and system stats
func SuperAdminDashboardHandler(w http.ResponseWriter, r *http.Request) {
	claims, ok := r.Context().Value("userClaims").(*models.Claims)
	if !ok || !utils.IsSuperAdmin(claims.Role) {
		RenderTemplate(w, r, "error.html", models.PageData{Title: "Forbidden", ErrorMessage: "You do not have permission to view this page."})
		return
	}

	// Fetch super admin user details
	adminUser, err := database.Queries.GetUserByID(context.Background(), pgtype.UUID{Bytes: claims.UserID, Valid: true})
	if err != nil {
		RenderTemplate(w, r, "error.html", models.PageData{Title: "Error", ErrorMessage: "Could not retrieve admin user details."})
		return
	}

	// Get all users
	allUsers, err := database.Queries.GetAllUsers(context.Background())
	if err != nil {
		log.Printf("Error fetching all users: %v", err)
		RenderTemplate(w, r, "error.html", models.PageData{Title: "Error", ErrorMessage: "Could not retrieve users."})
		return
	}

	// Get role counts
	roleCounts, err := database.Queries.CountUsersByRole(context.Background())
	if err != nil {
		log.Printf("Error fetching role counts: %v", err)
	}

	// Get all appointments
	allAppointments, err := database.Queries.GetAllAppointments(context.Background())
	if err != nil {
		log.Printf("Error fetching appointments: %v", err)
		RenderTemplate(w, r, "error.html", models.PageData{Title: "Error", ErrorMessage: "Could not retrieve appointments."})
		return
	}

	// Get all cars
	allCars, err := database.Queries.GetAllCarsWithUsers(context.Background())
	if err != nil {
		log.Printf("Error fetching cars: %v", err)
		RenderTemplate(w, r, "error.html", models.PageData{Title: "Error", ErrorMessage: "Could not retrieve cars."})
		return
	}

	// Calculate statistics
	totalUsers := len(allUsers)
	totalAppointments := len(allAppointments)
	totalCars := len(allCars)

	// Count appointments by status
	acceptedCount := 0
	pendingCount := 0
	cancelledCount := 0
	for _, appointment := range allAppointments {
		switch appointment.Status {
		case "confirmed":
			acceptedCount++
		case "pending":
			pendingCount++
		case "cancelled":
			cancelledCount++
		}
	}

	// Create role count map for easy access
	roleCountMap := make(map[string]int)
	for _, rc := range roleCounts {
		roleCountMap[rc.Role] = int(rc.Count)
	}

	data := models.PageData{
		Title:                 "Super Admin Dashboard",
		IsAuthenticated:       true,
		User:                  &adminUser,
		AllUsers:              allUsers,
		AllAppointments:       allAppointments,
		AllCars:               allCars,
		TotalUsers:            totalUsers,
		TotalCars:             totalCars,
		TotalAppointments:     totalAppointments,
		AcceptedAppointments:  acceptedCount,
		PendingAppointments:   pendingCount,
		CancelledAppointments: cancelledCount,
		RoleStats:             roleCountMap,
		MetaDescription: "Super Admin Dashboard - Manage all users, appointments, and system settings",
		CanonicalURL:    "/super-admin/dashboard",
	}

	RenderTemplate(w, r, "super_admin_dashboard.html", data)
}

// SuperAdminUsersHandler handles user management for super admins
func SuperAdminUsersHandler(w http.ResponseWriter, r *http.Request) {
	claims, ok := r.Context().Value("userClaims").(*models.Claims)
	if !ok || !utils.IsSuperAdmin(claims.Role) {
		RenderTemplate(w, r, "error.html", models.PageData{Title: "Forbidden", ErrorMessage: "You do not have permission to view this page."})
		return
	}

	// Get all users
	allUsers, err := database.Queries.GetAllUsers(context.Background())
	if err != nil {
		log.Printf("Error fetching all users: %v", err)
		RenderTemplate(w, r, "error.html", models.PageData{Title: "Error", ErrorMessage: "Could not retrieve users."})
		return
	}

	// Fetch super admin user details
	adminUser, err := database.Queries.GetUserByID(context.Background(), pgtype.UUID{Bytes: claims.UserID, Valid: true})
	if err != nil {
		RenderTemplate(w, r, "error.html", models.PageData{Title: "Error", ErrorMessage: "Could not retrieve admin user details."})
		return
	}

	data := models.PageData{
		Title:           "User Management",
		IsAuthenticated: true,
		User:            &adminUser,
		AllUsers:        allUsers,
		MetaDescription: "Super Admin User Management - View and manage all users",
		CanonicalURL:    "/super-admin/users",
	}

	RenderTemplate(w, r, "super_admin_users.html", data)
}

// SuperAdminUpdateUserRoleHandler handles role updates for users
func SuperAdminUpdateUserRoleHandler(w http.ResponseWriter, r *http.Request) {
	claims, ok := r.Context().Value("userClaims").(*models.Claims)
	if !ok || !utils.IsSuperAdmin(claims.Role) {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	userIDStr := r.FormValue("user_id")
	newRole := r.FormValue("role")

	if userIDStr == "" || newRole == "" {
		http.Error(w, "Missing required fields", http.StatusBadRequest)
		return
	}

	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		http.Error(w, "Invalid user ID", http.StatusBadRequest)
		return
	}

	// Validate role
	validRoles := []string{utils.RoleUser, utils.RoleAdmin, utils.RoleSuperAdmin}
	isValidRole := false
	for _, validRole := range validRoles {
		if newRole == validRole {
			isValidRole = true
			break
		}
	}
	if !isValidRole {
		http.Error(w, "Invalid role", http.StatusBadRequest)
		return
	}

	// Update user role
	err = database.Queries.UpdateUserRoleByID(context.Background(), db.UpdateUserRoleByIDParams{
		ID:   pgtype.UUID{Bytes: userID, Valid: true},
		Role: newRole,
	})
	if err != nil {
		log.Printf("Error updating user role: %v", err)
		http.Error(w, "Failed to update user role", http.StatusInternalServerError)
		return
	}

	// Blacklist all user tokens to force re-login
	database.BlacklistAllUserTokens(userID, "role_changed_by_superadmin")

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "success", "message": "User role updated successfully"})
}

// SuperAdminDeleteUserHandler handles user deletion
func SuperAdminDeleteUserHandler(w http.ResponseWriter, r *http.Request) {
	claims, ok := r.Context().Value("userClaims").(*models.Claims)
	if !ok || !utils.IsSuperAdmin(claims.Role) {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	userIDStr := r.FormValue("user_id")
	if userIDStr == "" {
		http.Error(w, "Missing user ID", http.StatusBadRequest)
		return
	}

	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		http.Error(w, "Invalid user ID", http.StatusBadRequest)
		return
	}

	// Prevent self-deletion
	if userID == claims.UserID {
		http.Error(w, "Cannot delete your own account", http.StatusBadRequest)
		return
	}

	// Blacklist all user tokens first
	database.BlacklistAllUserTokens(userID, "user_deleted_by_superadmin")

	// Delete user (cascades to delete appointments and cars)
	err = database.Queries.DeleteUserByID(context.Background(), pgtype.UUID{Bytes: userID, Valid: true})
	if err != nil {
		log.Printf("Error deleting user: %v", err)
		http.Error(w, "Failed to delete user", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "success", "message": "User deleted successfully"})
}

// SuperAdminActivateUserHandler handles user activation
func SuperAdminActivateUserHandler(w http.ResponseWriter, r *http.Request) {
	claims, ok := r.Context().Value("userClaims").(*models.Claims)
	if !ok || !utils.IsSuperAdmin(claims.Role) {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	userIDStr := r.FormValue("user_id")
	if userIDStr == "" {
		http.Error(w, "Missing user ID", http.StatusBadRequest)
		return
	}

	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		http.Error(w, "Invalid user ID", http.StatusBadRequest)
		return
	}

	// Activate user by setting email_verified to true
	err = database.Queries.VerifyUserEmail(context.Background(), pgtype.UUID{Bytes: userID, Valid: true})
	if err != nil {
		log.Printf("Error activating user: %v", err)
		http.Error(w, "Failed to activate user", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "success", "message": "User activated successfully"})
}

// SuperAdminSendPasswordResetHandler sends a password reset email to a user
func SuperAdminSendPasswordResetHandler(w http.ResponseWriter, r *http.Request) {
	claims, ok := r.Context().Value("userClaims").(*models.Claims)
	if !ok || !utils.IsSuperAdmin(claims.Role) {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	email := r.FormValue("email")
	if email == "" {
		http.Error(w, "Missing email", http.StatusBadRequest)
		return
	}

	// Generate password reset token
	token := utils.GenerateSecureToken()
	expiresAt := utils.GetPasswordResetExpiry()

	// Set the token in the database
	err := database.Queries.SetPasswordResetToken(context.Background(), db.SetPasswordResetTokenParams{
		Email:                email,
		PasswordResetToken:   pgtype.Text{String: token, Valid: true},
		PasswordResetExpires: pgtype.Timestamptz{Time: expiresAt, Valid: true},
	})
	if err != nil {
		log.Printf("Error setting password reset token: %v", err)
		http.Error(w, "Failed to set password reset token", http.StatusInternalServerError)
		return
	}

	// Send password reset email
	emailService := utils.NewEmailService()
	err = emailService.SendPasswordResetEmail(email, token)
	if err != nil {
		log.Printf("Error sending password reset email: %v", err)
		http.Error(w, "Failed to send password reset email", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "success", "message": "Password reset email sent successfully"})
}

// SuperAdminVehicleManagementHandler handles vehicle allocation and management
func SuperAdminVehicleManagementHandler(w http.ResponseWriter, r *http.Request) {
	claims, ok := r.Context().Value("userClaims").(*models.Claims)
	if !ok || !utils.IsSuperAdmin(claims.Role) {
		RenderTemplate(w, r, "error.html", models.PageData{Title: "Forbidden", ErrorMessage: "You do not have permission to view this page."})
		return
	}

	// Get all cars with user information
	allCars, err := database.Queries.GetAllCarsWithUsers(context.Background())
	if err != nil {
		log.Printf("Error fetching cars: %v", err)
		RenderTemplate(w, r, "error.html", models.PageData{Title: "Error", ErrorMessage: "Could not retrieve cars."})
		return
	}

	// Get all users for allocation dropdown
	allUsers, err := database.Queries.GetAllUsers(context.Background())
	if err != nil {
		log.Printf("Error fetching users: %v", err)
		RenderTemplate(w, r, "error.html", models.PageData{Title: "Error", ErrorMessage: "Could not retrieve users."})
		return
	}

	// Fetch super admin user details
	adminUser, err := database.Queries.GetUserByID(context.Background(), pgtype.UUID{Bytes: claims.UserID, Valid: true})
	if err != nil {
		RenderTemplate(w, r, "error.html", models.PageData{Title: "Error", ErrorMessage: "Could not retrieve admin user details."})
		return
	}

	data := models.PageData{
		Title:           "Vehicle Management",
		IsAuthenticated: true,
		User:            &adminUser,
		AllCars:         allCars,
		AllUsers:        allUsers,
		MetaDescription: "Super Admin Vehicle Management - Allocate and manage vehicles",
		CanonicalURL:    "/super-admin/vehicles",
	}

	RenderTemplate(w, r, "super_admin_vehicles.html", data)
}

// SuperAdminAllocateVehicleHandler handles vehicle allocation to users
func SuperAdminAllocateVehicleHandler(w http.ResponseWriter, r *http.Request) {
	claims, ok := r.Context().Value("userClaims").(*models.Claims)
	if !ok || !utils.IsSuperAdmin(claims.Role) {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	carIDStr := r.FormValue("car_id")
	userIDStr := r.FormValue("user_id")

	if carIDStr == "" || userIDStr == "" {
		http.Error(w, "Missing required fields", http.StatusBadRequest)
		return
	}

	carID, err := uuid.Parse(carIDStr)
	if err != nil {
		http.Error(w, "Invalid car ID", http.StatusBadRequest)
		return
	}

	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		http.Error(w, "Invalid user ID", http.StatusBadRequest)
		return
	}

	// Allocate car to user
	err = database.Queries.AllocateCarToUser(context.Background(), db.AllocateCarToUserParams{
		ID:     pgtype.UUID{Bytes: carID, Valid: true},
		UserID: pgtype.UUID{Bytes: userID, Valid: true},
	})
	if err != nil {
		log.Printf("Error allocating car to user: %v", err)
		http.Error(w, "Failed to allocate car", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "success", "message": "Vehicle allocated successfully"})
}

// SuperAdminRemoveVehicleHandler handles vehicle removal
func SuperAdminRemoveVehicleHandler(w http.ResponseWriter, r *http.Request) {
	claims, ok := r.Context().Value("userClaims").(*models.Claims)
	if !ok || !utils.IsSuperAdmin(claims.Role) {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	carIDStr := r.FormValue("car_id")
	if carIDStr == "" {
		http.Error(w, "Missing car ID", http.StatusBadRequest)
		return
	}

	carID, err := uuid.Parse(carIDStr)
	if err != nil {
		http.Error(w, "Invalid car ID", http.StatusBadRequest)
		return
	}

	// Delete car (this will also delete associated appointments due to foreign key constraints)
	err = database.Queries.AdminDeleteCar(context.Background(), pgtype.UUID{Bytes: carID, Valid: true})
	if err != nil {
		log.Printf("Error deleting car: %v", err)
		http.Error(w, "Failed to delete car", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "success", "message": "Vehicle removed successfully"})
}

// SuperAdminSystemStatsHandler returns system statistics as JSON
func SuperAdminSystemStatsHandler(w http.ResponseWriter, r *http.Request) {
	claims, ok := r.Context().Value("userClaims").(*models.Claims)
	if !ok || !utils.IsSuperAdmin(claims.Role) {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	// Get role counts
	roleCounts, err := database.Queries.CountUsersByRole(context.Background())
	if err != nil {
		log.Printf("Error fetching role counts: %v", err)
		http.Error(w, "Failed to fetch statistics", http.StatusInternalServerError)
		return
	}

	// Get all appointments for status counts
	allAppointments, err := database.Queries.GetAllAppointments(context.Background())
	if err != nil {
		log.Printf("Error fetching appointments: %v", err)
		http.Error(w, "Failed to fetch statistics", http.StatusInternalServerError)
		return
	}

	// Count appointments by status
	appointmentStats := map[string]int{
		"confirmed": 0,
		"pending":   0,
		"cancelled": 0,
	}

	for _, appointment := range allAppointments {
		appointmentStats[appointment.Status]++
	}

	// Get total cars
	allCars, err := database.Queries.GetAllCarsWithUsers(context.Background())
	if err != nil {
		log.Printf("Error fetching cars: %v", err)
		http.Error(w, "Failed to fetch statistics", http.StatusInternalServerError)
		return
	}

	stats := map[string]interface{}{
		"roles":        roleCounts,
		"appointments": appointmentStats,
		"totalCars":    len(allCars),
		"totalUsers":   0, // Will be calculated from role counts
	}

	// Calculate total users
	totalUsers := 0
	for _, rc := range roleCounts {
		totalUsers += int(rc.Count)
	}
	stats["totalUsers"] = totalUsers

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(stats)
}

// SuperAdminToggleDashboardHandler allows super admin to switch between admin and super admin dashboards
func SuperAdminToggleDashboardHandler(w http.ResponseWriter, r *http.Request) {
	claims, ok := r.Context().Value("userClaims").(*models.Claims)
	if !ok || !utils.IsSuperAdmin(claims.Role) {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	// Get the target dashboard from query parameter
	target := r.URL.Query().Get("target")

	switch target {
	case "admin":
		http.Redirect(w, r, "/admin/dashboard", http.StatusSeeOther)
	case "super":
		http.Redirect(w, r, "/super-admin/dashboard", http.StatusSeeOther)
	default:
		// Default to super admin dashboard
		http.Redirect(w, r, "/super-admin/dashboard", http.StatusSeeOther)
	}
}

// SuperAdminViewUserHandler allows super admin to view a user's dashboard/garage page
func SuperAdminViewUserHandler(w http.ResponseWriter, r *http.Request) {
	claims, ok := r.Context().Value("userClaims").(*models.Claims)
	if !ok || !utils.IsSuperAdmin(claims.Role) {
		RenderTemplate(w, r, "error.html", models.PageData{Title: "Forbidden", ErrorMessage: "You do not have permission to view this page."})
		return
	}

	// Get user ID from query parameter
	userIDStr := r.URL.Query().Get("user_id")
	if userIDStr == "" {
		RenderTemplate(w, r, "error.html", models.PageData{Title: "Error", ErrorMessage: "Missing user ID."})
		return
	}

	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		RenderTemplate(w, r, "error.html", models.PageData{Title: "Error", ErrorMessage: "Invalid user ID."})
		return
	}

	// Get the user whose page we're viewing
	targetUser, err := database.Queries.GetUserByID(context.Background(), pgtype.UUID{Bytes: userID, Valid: true})
	if err != nil {
		log.Printf("Error fetching target user: %v", err)
		RenderTemplate(w, r, "error.html", models.PageData{Title: "Error", ErrorMessage: "Could not find user."})
		return
	}

	// Get user's cars
	cars, err := database.Queries.GetCarsByUserID(context.Background(), pgtype.UUID{Bytes: userID, Valid: true})
	if err != nil {
		log.Printf("Error fetching user cars: %v", err)
		cars = []db.Car{} // Empty slice if error
	}

	// Get user's appointments
	appointments, err := database.Queries.GetAppointmentsForUser(context.Background(), pgtype.UUID{Bytes: userID, Valid: true})
	if err != nil {
		log.Printf("Error fetching user appointments: %v", err)
		appointments = []db.GetAppointmentsForUserRow{} // Empty slice if error
	}

	// Parse success/error messages from query params
	successMsg := r.URL.Query().Get("success")
	errorMsg := r.URL.Query().Get("error")

	// Build data for garage page
	data := models.PageData{
		Title:            "Viewing: " + targetUser.Name,
		IsAuthenticated:  true,
		User:             &targetUser,      // Show the target user's info
		UserCars:         cars,              // User's cars
		UserAppointments: appointments,      // User's appointments
		Success:          successMsg,
		Error:            errorMsg,
	}

	// Render the super admin user view template (modified garage view)
	RenderTemplate(w, r, "super_admin_user_view.html", data)
}

// SuperAdminAddCarForUserHandler allows super admin to add a car for a specific user
func SuperAdminAddCarForUserHandler(w http.ResponseWriter, r *http.Request) {
	claims, ok := r.Context().Value("userClaims").(*models.Claims)
	if !ok || !utils.IsSuperAdmin(claims.Role) {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Get target user ID and registration number
	userIDStr := r.FormValue("user_id")
	registrationNumber := r.FormValue("registrationNumber")

	if userIDStr == "" || registrationNumber == "" {
		http.Error(w, "Missing required fields", http.StatusBadRequest)
		return
	}

	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		http.Error(w, "Invalid user ID", http.StatusBadRequest)
		return
	}

	// Sanitize registration number
	registrationNumber = strings.ToUpper(strings.TrimSpace(registrationNumber))

	// Check if car already exists
	_, err = database.Queries.GetCarByRegistration(context.Background(), registrationNumber)
	if err == nil {
		http.Redirect(w, r, "/super-admin/view-user?user_id="+userIDStr+"&error=Vehicle already exists", http.StatusSeeOther)
		return
	}

	// Fetch vehicle data from DVLA
	dvlaClient := utils.NewDVLAClient()
	dvlaResp, err := dvlaClient.GetVehicleData(registrationNumber)
	if err != nil {
		log.Printf("Error fetching DVLA data: %v", err)
		http.Redirect(w, r, "/super-admin/view-user?user_id="+userIDStr+"&error=Could not fetch vehicle data from DVLA", http.StatusSeeOther)
		return
	}

	// Convert DVLA response to car data
	carData, err := convertDVLAResponseToCarData(userID, dvlaResp)
	if err != nil {
		log.Printf("Error converting DVLA data: %v", err)
		http.Redirect(w, r, "/super-admin/view-user?user_id="+userIDStr+"&error=Error processing vehicle data", http.StatusSeeOther)
		return
	}

	// Create the car record
	_, err = database.Queries.CreateCar(context.Background(), db.CreateCarParams{
		UserID:                       pgtype.UUID{Bytes: carData.UserID, Valid: true},
		RegistrationNumber:           carData.RegistrationNumber,
		Make:                         convertStringPtr(carData.Make),
		Colour:                       convertStringPtr(carData.Colour),
		FuelType:                     convertStringPtr(carData.FuelType),
		EngineCapacity:               convertInt32Ptr(carData.EngineCapacity),
		YearOfManufacture:            convertInt32Ptr(carData.YearOfManufacture),
		MonthOfFirstRegistration:     convertStringPtr(carData.MonthOfFirstRegistration),
		MonthOfFirstDvlaRegistration: convertStringPtr(carData.MonthOfFirstDVLARegistration),
		TaxStatus:                    convertStringPtr(carData.TaxStatus),
		TaxDueDate:                   convertTimeToDate(carData.TaxDueDate),
		MotStatus:                    convertStringPtr(carData.MOTStatus),
		MotExpiryDate:                convertTimeToDate(carData.MOTExpiryDate),
		Co2Emissions:                 convertInt32Ptr(carData.CO2Emissions),
		EuroStatus:                   convertStringPtr(carData.EuroStatus),
		RealDrivingEmissions:         convertStringPtr(carData.RealDrivingEmissions),
		RevenueWeight:                convertInt32Ptr(carData.RevenueWeight),
		TypeApproval:                 convertStringPtr(carData.TypeApproval),
		Wheelplan:                    convertStringPtr(carData.Wheelplan),
		AutomatedVehicle:             convertBoolPtr(carData.AutomatedVehicle),
		MarkedForExport:              convertBoolPtr(carData.MarkedForExport),
		DateOfLastV5cIssued:          convertTimeToDate(carData.DateOfLastV5CIssued),
		ArtEndDate:                   convertTimeToDate(carData.ArtEndDate),
		DvlaDataFetchedAt:            pgtype.Timestamptz{Time: carData.DVLADataFetchedAt, Valid: true},
	})

	if err != nil {
		log.Printf("Error creating car record: %v", err)
		http.Redirect(w, r, "/super-admin/view-user?user_id="+userIDStr+"&error=Error saving vehicle", http.StatusSeeOther)
		return
	}

	// Redirect back to user view page
	http.Redirect(w, r, "/super-admin/view-user?user_id="+userIDStr+"&success=Vehicle added successfully", http.StatusSeeOther)
}

// SuperAdminDeleteCarForUserHandler allows super admin to delete a car for a specific user
func SuperAdminDeleteCarForUserHandler(w http.ResponseWriter, r *http.Request) {
	claims, ok := r.Context().Value("userClaims").(*models.Claims)
	if !ok || !utils.IsSuperAdmin(claims.Role) {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Get car ID and user ID
	carIDStr := r.FormValue("car_id")
	userIDStr := r.FormValue("user_id")

	if carIDStr == "" || userIDStr == "" {
		http.Error(w, "Missing required fields", http.StatusBadRequest)
		return
	}

	carID, err := uuid.Parse(carIDStr)
	if err != nil {
		http.Error(w, "Invalid car ID", http.StatusBadRequest)
		return
	}

	// Delete the car
	err = database.Queries.AdminDeleteCar(context.Background(), pgtype.UUID{Bytes: carID, Valid: true})
	if err != nil {
		log.Printf("Error deleting car: %v", err)
		http.Redirect(w, r, "/super-admin/view-user?user_id="+userIDStr+"&error=Could not delete vehicle", http.StatusSeeOther)
		return
	}

	// Redirect back to user view page
	http.Redirect(w, r, "/super-admin/view-user?user_id="+userIDStr+"&success=Vehicle deleted successfully", http.StatusSeeOther)
}