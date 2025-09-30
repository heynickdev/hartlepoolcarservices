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

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
)

// generateCarStatsChartData calculates vehicle registration statistics for the last 12 months
func generateCarStatsChartData(cars []db.Car) models.ChartData {
	now := time.Now()
	chartData := models.ChartData{
		Labels:    make([]string, 12),
		Confirmed: make([]int, 12), // Repurpose as total cars
		Pending:   make([]int, 12), // Tax due cars
		Cancelled: make([]int, 12), // MOT due cars
	}

	for i := 0; i < 12; i++ {
		month := now.AddDate(0, -i, 0)
		chartData.Labels[11-i] = month.Format("Jan")
		for _, car := range cars {
			if car.CreatedAt.Time.Year() == month.Year() && car.CreatedAt.Time.Month() == month.Month() {
				chartData.Confirmed[11-i]++ // Count registered cars
			}
			// Check for tax due this month
			if car.TaxDueDate.Valid && car.TaxDueDate.Time.Year() == month.Year() && car.TaxDueDate.Time.Month() == month.Month() {
				chartData.Pending[11-i]++
			}
			// Check for MOT due this month
			if car.MotExpiryDate.Valid && car.MotExpiryDate.Time.Year() == month.Year() && car.MotExpiryDate.Time.Month() == month.Month() {
				chartData.Cancelled[11-i]++
			}
		}
	}
	return chartData
}

func DashboardHandler(w http.ResponseWriter, r *http.Request) {
	claims, ok := r.Context().Value("userClaims").(*models.Claims)
	if !ok {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	// Redirect admins to appropriate dashboard
	if utils.IsSuperAdmin(claims.Role) {
		http.Redirect(w, r, "/super-admin/dashboard", http.StatusSeeOther)
		return
	} else if utils.IsAdmin(claims.Role) {
		http.Redirect(w, r, "/admin/dashboard", http.StatusSeeOther)
		return
	}

	user, err := database.Queries.GetUserByID(context.Background(), pgtype.UUID{Bytes: claims.UserID, Valid: true})
	if err != nil {
		log.Printf("Error fetching user: %v", err)
		http.Error(w, "Error loading user data", http.StatusInternalServerError)
		return
	}

	// Get user's cars
	cars, err := database.Queries.GetCarsByUserID(context.Background(), pgtype.UUID{Bytes: claims.UserID, Valid: true})
	if err != nil {
		log.Printf("Error fetching user cars: %v", err)
		cars = []db.Car{}
	}

	// Get user's appointments with car info
	userAppointments, err := database.Queries.GetAppointmentsForUser(context.Background(), pgtype.UUID{Bytes: claims.UserID, Valid: true})
	if err != nil {
		log.Printf("Error fetching user appointments: %v", err)
		userAppointments = []db.GetAppointmentsForUserRow{}
	}

	// Create a map of car ID to next appointment for quick lookup
	carNextAppointments := make(map[uuid.UUID]*db.Appointment)
	for _, car := range cars {
		nextAppt, err := database.Queries.GetNextAppointmentForCar(context.Background(), pgtype.UUID{Bytes: car.ID.Bytes, Valid: true})
		if err == nil {
			carNextAppointments[car.ID.Bytes] = &nextAppt
		}
	}

	// Calculate car statistics
	totalCars := len(cars)
	taxDueSoon := 0
	motDueSoon := 0
	now := time.Now()
	oneMonthFromNow := now.AddDate(0, 1, 0)

	for _, car := range cars {
		// Check if tax is due within next month
		if car.TaxDueDate.Valid && car.TaxDueDate.Time.Before(oneMonthFromNow) && car.TaxDueDate.Time.After(now) {
			taxDueSoon++
		}
		// Check if MOT is due within next month
		if car.MotExpiryDate.Valid && car.MotExpiryDate.Time.Before(oneMonthFromNow) && car.MotExpiryDate.Time.After(now) {
			motDueSoon++
		}
	}

	chartData := generateCarStatsChartData(cars)

	data := models.PageData{
		Title:                 "My Garage",
		IsAuthenticated:       true,
		User:                  &user,
		UserCars:              cars,
		TotalCars:             totalCars,
		PendingAppointments:   taxDueSoon, // Repurpose for tax due
		CancelledAppointments: motDueSoon, // Repurpose for MOT due
		ChartData:             chartData,
		UserAppointments:      userAppointments,
		CarNextAppointments:   carNextAppointments,
		Success:               r.URL.Query().Get("success"),
		Error:                 r.URL.Query().Get("error"),
	}

	RenderTemplate(w, r, "garage.html", data)
}

func VehicleDetailHandler(w http.ResponseWriter, r *http.Request) {
	claims, ok := r.Context().Value("userClaims").(*models.Claims)
	if !ok {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	// Get car ID from URL
	carIDStr := r.URL.Query().Get("id")
	if carIDStr == "" {
		http.Redirect(w, r, "/dashboard", http.StatusSeeOther)
		return
	}

	carID, err := uuid.Parse(carIDStr)
	if err != nil {
		http.Redirect(w, r, "/dashboard", http.StatusSeeOther)
		return
	}

	// Get the car and verify ownership
	car, err := database.Queries.GetCarByID(context.Background(), pgtype.UUID{Bytes: carID, Valid: true})
	if err != nil {
		log.Printf("Error fetching car: %v", err)
		http.Redirect(w, r, "/dashboard?error=Car+not+found", http.StatusSeeOther)
		return
	}

	// Verify ownership
	if car.UserID.Bytes != claims.UserID {
		http.Redirect(w, r, "/dashboard?error=Unauthorized", http.StatusSeeOther)
		return
	}

	// Get appointments for this car
	appointments, err := database.Queries.GetAppointmentsForCar(context.Background(), pgtype.UUID{Bytes: carID, Valid: true})
	if err != nil {
		log.Printf("Error fetching car appointments: %v", err)
		appointments = []db.Appointment{}
	}

	// Calculate appointment statistics
	total := len(appointments)
	accepted := 0
	pending := 0
	cancelled := 0

	for _, appt := range appointments {
		switch appt.Status {
		case "confirmed", "completed":
			accepted++
		case "pending":
			pending++
		case "cancelled":
			cancelled++
		}
	}

	data := models.PageData{
		Title:                 car.RegistrationNumber + " - Vehicle Details",
		IsAuthenticated:       true,
		SelectedCar:           &car,
		CarAppointments:       appointments,
		TotalAppointments:     total,
		AcceptedAppointments:  accepted,
		PendingAppointments:   pending,
		CancelledAppointments: cancelled,
		Success:               r.URL.Query().Get("success"),
		Error:                 r.URL.Query().Get("error"),
	}

	RenderTemplate(w, r, "vehicle_detail.html", data)
}

func CreateAppointmentHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Redirect(w, r, "/dashboard", http.StatusSeeOther)
		return
	}

	claims, ok := r.Context().Value("userClaims").(*models.Claims)
	if !ok {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	// Parse form data
	carIDStr := r.FormValue("carId")
	datetimeStr := r.FormValue("datetime")
	title := r.FormValue("title")
	description := r.FormValue("description")

	if carIDStr == "" || datetimeStr == "" || title == "" {
		http.Redirect(w, r, "/dashboard?error=Missing+required+fields", http.StatusSeeOther)
		return
	}

	carID, err := uuid.Parse(carIDStr)
	if err != nil {
		http.Redirect(w, r, "/dashboard?error=Invalid+car+ID", http.StatusSeeOther)
		return
	}

	datetime, err := time.Parse("2006-01-02T15:04", datetimeStr)
	if err != nil {
		http.Redirect(w, r, "/dashboard?error=Invalid+date+format", http.StatusSeeOther)
		return
	}

	// Validate appointment datetime restrictions
	now := time.Now()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())

	// Check if appointment is not in the past (must be today or later)
	if datetime.Before(today) {
		http.Redirect(w, r, "/vehicle?id="+carIDStr+"&error=Cannot+schedule+appointments+in+the+past", http.StatusSeeOther)
		return
	}

	// Check if appointment is not on a Sunday (Sunday = 0)
	if datetime.Weekday() == time.Sunday {
		http.Redirect(w, r, "/vehicle?id="+carIDStr+"&error=Cannot+schedule+appointments+on+Sundays", http.StatusSeeOther)
		return
	}

	// Check if appointment time is between 8am and 6pm
	hour := datetime.Hour()
	if hour < 8 || hour >= 18 {
		http.Redirect(w, r, "/vehicle?id="+carIDStr+"&error=Appointments+must+be+between+8am+and+6pm", http.StatusSeeOther)
		return
	}

	// Verify car ownership
	car, err := database.Queries.GetCarByID(context.Background(), pgtype.UUID{Bytes: carID, Valid: true})
	if err != nil || car.UserID.Bytes != claims.UserID {
		http.Redirect(w, r, "/dashboard?error=Unauthorized+or+car+not+found", http.StatusSeeOther)
		return
	}

	// Create appointment
	_, err = database.Queries.CreateAppointment(context.Background(), db.CreateAppointmentParams{
		UserID:      pgtype.UUID{Bytes: claims.UserID, Valid: true},
		CarID:       pgtype.UUID{Bytes: carID, Valid: true},
		Datetime:    pgtype.Timestamptz{Time: datetime, Valid: true},
		Title:       title,
		Description: pgtype.Text{String: description, Valid: description != ""},
	})

	if err != nil {
		log.Printf("Error creating appointment: %v", err)
		http.Redirect(w, r, "/vehicle?id="+carIDStr+"&error=Error+creating+appointment", http.StatusSeeOther)
		return
	}

	// Send email notification to admin
	user, err := database.Queries.GetUserByID(context.Background(), pgtype.UUID{Bytes: claims.UserID, Valid: true})
	if err == nil {
		emailService := utils.NewEmailService()
		descriptionText := description
		if descriptionText == "" {
			descriptionText = "No description provided"
		}
		formattedDateTime := datetime.Format("Monday, January 2, 2006 at 3:04 PM")

		carMake := car.Make.String
		if !car.Make.Valid || carMake == "" {
			carMake = "Unknown"
		}

		err = emailService.SendAppointmentNotification(
			user.Name,
			user.Email,
			car.RegistrationNumber,
			carMake,
			title,
			descriptionText,
			formattedDateTime,
		)
		if err != nil {
			log.Printf("Error sending appointment notification email: %v", err)
			// Don't fail the appointment creation if email fails
		}
	}

	// Redirect back to vehicle detail page
	http.Redirect(w, r, "/vehicle?id="+carIDStr+"&success=Appointment+created+successfully", http.StatusSeeOther)
}

func UserCancelAppointmentHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	claims, ok := r.Context().Value("userClaims").(*models.Claims)
	if !ok {
		http.Error(w, "User claims not found", http.StatusInternalServerError)
		return
	}

	appointmentIDStr := r.FormValue("appointmentId")
	carIDStr := r.FormValue("carId")

	if appointmentIDStr == "" {
		http.Error(w, "Appointment ID is required", http.StatusBadRequest)
		return
	}

	appointmentID, err := uuid.Parse(appointmentIDStr)
	if err != nil {
		http.Error(w, "Invalid appointment ID", http.StatusBadRequest)
		return
	}

	err = database.Queries.UserCancelAppointment(context.Background(), db.UserCancelAppointmentParams{
		ID:     pgtype.UUID{Bytes: appointmentID, Valid: true},
		UserID: pgtype.UUID{Bytes: claims.UserID, Valid: true},
	})

	if err != nil {
		log.Printf("Error cancelling appointment: %v", err)
		if carIDStr != "" {
			http.Redirect(w, r, "/vehicle?id="+carIDStr+"&error=Error+cancelling+appointment", http.StatusSeeOther)
		} else {
			http.Redirect(w, r, "/dashboard?error=Error+cancelling+appointment", http.StatusSeeOther)
		}
		return
	}

	if carIDStr != "" {
		http.Redirect(w, r, "/vehicle?id="+carIDStr+"&success=Appointment+cancelled+successfully", http.StatusSeeOther)
	} else {
		http.Redirect(w, r, "/dashboard?success=Appointment+cancelled+successfully", http.StatusSeeOther)
	}
}
