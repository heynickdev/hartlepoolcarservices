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
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
)

func AdminDashboardHandler(w http.ResponseWriter, r *http.Request) {
	claims, ok := r.Context().Value("userClaims").(*models.Claims)
	if !ok || !utils.IsAdmin(claims.Role) {
		RenderTemplate(w, r, "error.html", models.PageData{Title: "Forbidden", ErrorMessage: "You do not have permission to view this page."})
		return
	}

	// Fetch admin user details
	adminUser, err := database.Queries.GetUserByID(context.Background(), pgtype.UUID{Bytes: claims.UserID, Valid: true})
	if err != nil {
		RenderTemplate(w, r, "error.html", models.PageData{Title: "Error", ErrorMessage: "Could not retrieve admin user details."})
		return
	}

	// Get all cars with user information
	allCars, err := database.Queries.GetAllCarsWithUsers(context.Background())
	if err != nil {
		log.Printf("Error getting all cars: %v", err)
		allCars = []db.GetAllCarsWithUsersRow{}
	}

	// Get all appointments
	allAppointments, err := database.Queries.GetAllAppointments(context.Background())
	if err != nil {
		log.Printf("Error getting all appointments: %v", err)
		allAppointments = []db.GetAllAppointmentsRow{}
	}

	// Calculate statistics
	totalCars := len(allCars)
	totalAppointments := len(allAppointments)
	pendingAppointments := 0
	completedAppointments := 0

	for _, appt := range allAppointments {
		switch appt.Status {
		case "pending":
			pendingAppointments++
		case "completed", "confirmed":
			completedAppointments++
		}
	}

	// Group cars by user
	carsByUser := make(map[uuid.UUID]models.UserCarSummary)
	for _, car := range allCars {
		userID := car.UserID
		if summary, exists := carsByUser[userID.Bytes]; exists {
			summary.Count++
			carsByUser[userID.Bytes] = summary
		} else {
			user := db.User{
				ID:    userID,
				Name:  car.UserName,
				Email: car.UserEmail,
			}
			carsByUser[userID.Bytes] = models.UserCarSummary{
				User:  user,
				Count: 1,
			}
		}
	}

	data := models.PageData{
		Title:                "Admin Dashboard",
		IsAuthenticated:      true,
		User:                 &adminUser,
		AllCars:              allCars,
		AllAppointments:      allAppointments,
		TotalCars:            totalCars,
		TotalAppointments:    totalAppointments,
		PendingAppointments:  pendingAppointments,
		AcceptedAppointments: completedAppointments,
		CarsByUser:           carsByUser,
		Success:              r.URL.Query().Get("success"),
		Error:                r.URL.Query().Get("error"),
	}

	RenderTemplate(w, r, "admin_dashboard.html", data)
}

func AdminOverviewHandler(w http.ResponseWriter, r *http.Request) {
	claims, ok := r.Context().Value("userClaims").(*models.Claims)
	if !ok || !utils.IsAdmin(claims.Role) {
		RenderTemplate(w, r, "error.html", models.PageData{Title: "Forbidden", ErrorMessage: "You do not have permission to view this page."})
		return
	}

	// Fetch admin user details
	adminUser, err := database.Queries.GetUserByID(context.Background(), pgtype.UUID{Bytes: claims.UserID, Valid: true})
	if err != nil {
		RenderTemplate(w, r, "error.html", models.PageData{Title: "Error", ErrorMessage: "Could not retrieve admin user details."})
		return
	}

	// Get all cars
	allCars, err := database.Queries.GetAllCarsWithUsers(context.Background())
	if err != nil {
		log.Printf("Error getting all cars: %v", err)
		allCars = []db.GetAllCarsWithUsersRow{}
	}

	// Get all appointments
	allAppointments, err := database.Queries.GetAllAppointments(context.Background())
	if err != nil {
		log.Printf("Error getting all appointments: %v", err)
		allAppointments = []db.GetAllAppointmentsRow{}
	}

	// Calculate statistics
	totalCars := len(allCars)
	totalAppointments := len(allAppointments)

	pendingAppointments := 0
	confirmedAppointments := 0
	completedAppointments := 0
	cancelledAppointments := 0

	for _, appt := range allAppointments {
		switch appt.Status {
		case "pending":
			pendingAppointments++
		case "confirmed":
			confirmedAppointments++
		case "completed":
			completedAppointments++
		case "cancelled":
			cancelledAppointments++
		}
	}

	// Group cars by user
	carsByUser := make(map[uuid.UUID]models.UserCarSummary)
	for _, car := range allCars {
		userID := car.UserID
		if summary, exists := carsByUser[userID.Bytes]; exists {
			summary.Count++
			carsByUser[userID.Bytes] = summary
		} else {
			user := db.User{
				ID:    userID,
				Name:  car.UserName,
				Email: car.UserEmail,
			}
			carsByUser[userID.Bytes] = models.UserCarSummary{
				User:  user,
				Count: 1,
			}
		}
	}

	// Generate chart data for overview
	chartData := generateAdminChartData(allCars, allAppointments)

	data := models.PageData{
		Title:                 "Admin Overview",
		IsAuthenticated:       true,
		User:                  &adminUser,
		AllCars:               allCars,
		AllAppointments:       allAppointments,
		TotalCars:             totalCars,
		TotalAppointments:     totalAppointments,
		PendingAppointments:   pendingAppointments,
		AcceptedAppointments:  confirmedAppointments + completedAppointments,
		CancelledAppointments: cancelledAppointments,
		ChartData:             chartData,
		CarsByUser:            carsByUser,
		Success:               r.URL.Query().Get("success"),
		Error:                 r.URL.Query().Get("error"),
	}

	RenderTemplate(w, r, "admin_overview.html", data)
}

func AdminUpdateAppointmentStatusHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	claims, ok := r.Context().Value("userClaims").(*models.Claims)
	if !ok || !utils.IsAdmin(claims.Role) {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	appointmentIDStr := r.FormValue("appointmentId")
	newStatus := r.FormValue("status")

	if appointmentIDStr == "" || newStatus == "" {
		http.Error(w, "Missing required fields", http.StatusBadRequest)
		return
	}

	appointmentID, err := uuid.Parse(appointmentIDStr)
	if err != nil {
		http.Error(w, "Invalid appointment ID", http.StatusBadRequest)
		return
	}

	err = database.Queries.UpdateAppointmentStatus(context.Background(), db.UpdateAppointmentStatusParams{
		ID:     pgtype.UUID{Bytes: appointmentID, Valid: true},
		Status: newStatus,
	})

	if err != nil {
		log.Printf("Error updating appointment status: %v", err)
		http.Redirect(w, r, "/admin/dashboard?error=Error+updating+appointment+status", http.StatusSeeOther)
		return
	}

	http.Redirect(w, r, "/admin/dashboard?success=Appointment+status+updated+successfully", http.StatusSeeOther)
}

func AdminDeleteAppointmentHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	claims, ok := r.Context().Value("userClaims").(*models.Claims)
	if !ok || !utils.IsAdmin(claims.Role) {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	appointmentIDStr := r.FormValue("appointmentId")
	if appointmentIDStr == "" {
		http.Error(w, "Appointment ID is required", http.StatusBadRequest)
		return
	}

	appointmentID, err := uuid.Parse(appointmentIDStr)
	if err != nil {
		http.Error(w, "Invalid appointment ID", http.StatusBadRequest)
		return
	}

	err = database.Queries.AdminDeleteAppointment(context.Background(), pgtype.UUID{Bytes: appointmentID, Valid: true})
	if err != nil {
		log.Printf("Error deleting appointment: %v", err)
		http.Redirect(w, r, "/admin/dashboard?error=Error+deleting+appointment", http.StatusSeeOther)
		return
	}

	http.Redirect(w, r, "/admin/dashboard?success=Appointment+deleted+successfully", http.StatusSeeOther)
}

func AdminCalendarHandler(w http.ResponseWriter, r *http.Request) {
	claims, ok := r.Context().Value("userClaims").(*models.Claims)
	if !ok || !utils.IsAdmin(claims.Role) {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	// Get appointments for calendar view
	allAppointments, err := database.Queries.GetAllAppointments(context.Background())
	if err != nil {
		log.Printf("Error getting appointments for calendar: %v", err)
		http.Error(w, "Error loading calendar data", http.StatusInternalServerError)
		return
	}

	// Convert to calendar format
	calendarEvents := make([]map[string]interface{}, 0)
	for _, appt := range allAppointments {
		event := map[string]interface{}{
			"id":    appt.ID.String(),
			"title": appt.Title,
			"start": appt.Datetime.Time.Format(time.RFC3339),
			"color": getStatusColor(appt.Status),
			"extendedProps": map[string]interface{}{
				"userName":  appt.UserName,
				"userEmail": appt.UserEmail,
				"status":    appt.Status,
				"carReg":    appt.CarRegistration.String,
				"carMake":   appt.CarMake.String,
			},
		}
		calendarEvents = append(calendarEvents, event)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(calendarEvents)
}

// Helper functions
func generateAdminChartData(cars []db.GetAllCarsWithUsersRow, appointments []db.GetAllAppointmentsRow) models.ChartData {
	now := time.Now()

	// Find the latest appointment month to determine chart range
	latestMonth := now
	for _, appt := range appointments {
		if appt.Datetime.Time.After(latestMonth) {
			latestMonth = appt.Datetime.Time
		}
	}

	// If latest appointment is more than 6 months in future, limit it
	maxFutureMonths := 6
	if latestMonth.After(now.AddDate(0, maxFutureMonths, 0)) {
		latestMonth = now.AddDate(0, maxFutureMonths, 0)
	}

	// Calculate start month (11 months before latest month)
	startMonth := latestMonth.AddDate(0, -11, 0)

	chartData := models.ChartData{
		Labels:    make([]string, 12),
		Confirmed: make([]int, 12), // Cars registered
		Pending:   make([]int, 12), // Pending appointments
		Cancelled: make([]int, 12), // Completed appointments
	}

	for i := 0; i < 12; i++ {
		month := startMonth.AddDate(0, i, 0)
		chartData.Labels[i] = month.Format("Jan")

		// Count cars registered this month
		for _, car := range cars {
			if car.CreatedAt.Time.Year() == month.Year() && car.CreatedAt.Time.Month() == month.Month() {
				chartData.Confirmed[i]++
			}
		}

		// Count appointments by status this month
		for _, appt := range appointments {
			if appt.Datetime.Time.Year() == month.Year() && appt.Datetime.Time.Month() == month.Month() {
				switch appt.Status {
				case "pending":
					chartData.Pending[i]++
				case "completed", "confirmed":
					chartData.Cancelled[i]++
				}
			}
		}
	}
	return chartData
}

func AdminVehicleDetailHandler(w http.ResponseWriter, r *http.Request) {
	claims, ok := r.Context().Value("userClaims").(*models.Claims)
	if !ok || !utils.IsAdmin(claims.Role) {
		RenderTemplate(w, r, "error.html", models.PageData{Title: "Forbidden", ErrorMessage: "You do not have permission to view this page."})
		return
	}

	// Get car ID from query parameter
	carIDStr := r.URL.Query().Get("id")
	if carIDStr == "" {
		RenderTemplate(w, r, "error.html", models.PageData{Title: "Error", ErrorMessage: "Car ID is required."})
		return
	}

	carID, err := uuid.Parse(carIDStr)
	if err != nil {
		RenderTemplate(w, r, "error.html", models.PageData{Title: "Error", ErrorMessage: "Invalid car ID."})
		return
	}

	// Fetch admin user details
	adminUser, err := database.Queries.GetUserByID(context.Background(), pgtype.UUID{Bytes: claims.UserID, Valid: true})
	if err != nil {
		RenderTemplate(w, r, "error.html", models.PageData{Title: "Error", ErrorMessage: "Could not retrieve admin user details."})
		return
	}

	// Get car details
	car, err := database.Queries.GetCarByID(context.Background(), pgtype.UUID{Bytes: carID, Valid: true})
	if err != nil {
		RenderTemplate(w, r, "error.html", models.PageData{Title: "Error", ErrorMessage: "Car not found."})
		return
	}

	// Get vehicle owner details
	vehicleOwner, err := database.Queries.GetUserByID(context.Background(), car.UserID)
	if err != nil {
		RenderTemplate(w, r, "error.html", models.PageData{Title: "Error", ErrorMessage: "Could not retrieve vehicle owner details."})
		return
	}

	// Get appointments for this car
	carAppointments, err := database.Queries.GetAppointmentsForCar(context.Background(), pgtype.UUID{Bytes: carID, Valid: true})
	if err != nil {
		log.Printf("Error getting appointments for car: %v", err)
		carAppointments = []db.Appointment{}
	}

	data := models.PageData{
		Title:           "Admin Vehicle Details",
		IsAuthenticated: true,
		User:            &adminUser,
		SelectedCar:     &car,
		VehicleOwner:    &vehicleOwner,
		CarAppointments: carAppointments,
		Success:         r.URL.Query().Get("success"),
		Error:           r.URL.Query().Get("error"),
	}

	RenderTemplate(w, r, "admin_vehicle_detail.html", data)
}

func AdminDeleteCarHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	claims, ok := r.Context().Value("userClaims").(*models.Claims)
	if !ok || !utils.IsAdmin(claims.Role) {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	carIDStr := r.FormValue("carId")
	if carIDStr == "" {
		http.Error(w, "Car ID is required", http.StatusBadRequest)
		return
	}

	carID, err := uuid.Parse(carIDStr)
	if err != nil {
		http.Error(w, "Invalid car ID", http.StatusBadRequest)
		return
	}

	// Delete all appointments for this car first
	_, err = database.Queries.GetAppointmentsForCar(context.Background(), pgtype.UUID{Bytes: carID, Valid: true})
	if err == nil {
		// If there are appointments, we should handle them (could cascade delete or restrict)
		// For now, let's delete the appointments first
		appointments, _ := database.Queries.GetAppointmentsForCar(context.Background(), pgtype.UUID{Bytes: carID, Valid: true})
		for _, appt := range appointments {
			database.Queries.AdminDeleteAppointment(context.Background(), appt.ID)
		}
	}

	err = database.Queries.AdminDeleteCar(context.Background(), pgtype.UUID{Bytes: carID, Valid: true})
	if err != nil {
		log.Printf("Error deleting car: %v", err)
		http.Redirect(w, r, "/admin/cars?error=Error+removing+vehicle", http.StatusSeeOther)
		return
	}

	// Check if we're coming from the vehicle detail page
	referer := r.Header.Get("Referer")
	if strings.Contains(referer, "/admin/vehicle") {
		http.Redirect(w, r, "/admin/dashboard?success=Vehicle+removed+successfully", http.StatusSeeOther)
	} else {
		http.Redirect(w, r, "/admin/cars?success=Vehicle+removed+successfully", http.StatusSeeOther)
	}
}

func getStatusColor(status string) string {
	switch status {
	case "pending":
		return "#f59e0b" // yellow
	case "confirmed":
		return "#3b82f6" // blue
	case "completed":
		return "#10b981" // green
	case "cancelled":
		return "#ef4444" // red
	default:
		return "#6b7280" // gray
	}
}
