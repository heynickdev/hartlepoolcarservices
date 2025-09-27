package handlers

import (
	"context"
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

func AddCarHandler(w http.ResponseWriter, r *http.Request) {
	claims, ok := r.Context().Value("userClaims").(*models.Claims)
	if !ok {
		http.Error(w, "User claims not found", http.StatusInternalServerError)
		return
	}

	if r.Method == "GET" {
		// Render the add car form
		data := models.PageData{
			Title:           "Add Car",
			IsAuthenticated: true,
		}
		RenderTemplate(w, r, "add_car.html", data)
		return
	}

	if r.Method == "POST" {
		// Parse form data
		if err := r.ParseForm(); err != nil {
			http.Error(w, "Invalid form data", http.StatusBadRequest)
			return
		}

		registrationNumber := strings.ToUpper(strings.TrimSpace(r.FormValue("registrationNumber")))
		if registrationNumber == "" {
			renderAddCarError(w, r, "Registration number is required")
			return
		}

		// Check if car already exists for this user
		_, err := database.Queries.GetCarByRegistration(context.Background(), registrationNumber)
		if err == nil {
			renderAddCarError(w, r, "Car with this registration number already exists")
			return
		}

		// Fetch data from DVLA API
		dvlaClient := utils.NewDVLAClient()
		dvlaResp, err := dvlaClient.GetVehicleData(registrationNumber)
		if err != nil {
			log.Printf("DVLA API error for registration %s: %v", registrationNumber, err)
			renderAddCarError(w, r, "Unable to fetch vehicle data from DVLA. Please check the registration number.")
			return
		}

		// Convert DVLA response to database format
		carData, err := convertDVLAResponseToCarData(claims.UserID, dvlaResp)
		if err != nil {
			log.Printf("Error converting DVLA data: %v", err)
			renderAddCarError(w, r, "Error processing vehicle data")
			return
		}

		// Create car record in database
		_, err = database.Queries.CreateCar(context.Background(), db.CreateCarParams{
			UserID:                       pgtype.UUID{Bytes: carData.UserID, Valid: true},
			RegistrationNumber:           carData.RegistrationNumber,
			Make:                         convertStringPtr(carData.Make),
			Colour:                       convertStringPtr(carData.Colour),
			FuelType:                     convertStringPtr(carData.FuelType),
			EngineCapacity:               convertInt32Ptr(carData.EngineCapacity),
			YearOfManufacture:           convertInt32Ptr(carData.YearOfManufacture),
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
			renderAddCarError(w, r, "Error saving car data")
			return
		}

		// Redirect to cars page with success message
		http.Redirect(w, r, "/dashboard?success=Car+added+successfully", http.StatusSeeOther)
	}
}

func DeleteCarHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	claims, ok := r.Context().Value("userClaims").(*models.Claims)
	if !ok {
		http.Error(w, "User claims not found", http.StatusInternalServerError)
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

	// Delete the car (only if it belongs to the user)
	err = database.Queries.DeleteCar(context.Background(), db.DeleteCarParams{
		ID:     pgtype.UUID{Bytes: carID, Valid: true},
		UserID: pgtype.UUID{Bytes: claims.UserID, Valid: true},
	})

	if err != nil {
		log.Printf("Error deleting car: %v", err)
		http.Redirect(w, r, "/cars?error=Error+deleting+car", http.StatusSeeOther)
		return
	}

	http.Redirect(w, r, "/dashboard?success=Car+deleted+successfully", http.StatusSeeOther)
}

func RefreshCarDataHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	claims, ok := r.Context().Value("userClaims").(*models.Claims)
	if !ok {
		http.Error(w, "User claims not found", http.StatusInternalServerError)
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

	// Get the car to verify ownership and get registration number
	car, err := database.Queries.GetCarByID(context.Background(), pgtype.UUID{Bytes: carID, Valid: true})
	if err != nil {
		http.Error(w, "Car not found", http.StatusNotFound)
		return
	}

	// Verify ownership
	if car.UserID.Bytes != claims.UserID {
		http.Error(w, "Unauthorized", http.StatusForbidden)
		return
	}

	// Fetch fresh data from DVLA API
	dvlaClient := utils.NewDVLAClient()
	dvlaResp, err := dvlaClient.GetVehicleData(car.RegistrationNumber)
	if err != nil {
		log.Printf("DVLA API error for registration %s: %v", car.RegistrationNumber, err)
		http.Redirect(w, r, "/dashboard?error=Unable+to+refresh+vehicle+data+from+DVLA", http.StatusSeeOther)
		return
	}

	// Convert and update car data
	carData, err := convertDVLAResponseToCarData(claims.UserID, dvlaResp)
	if err != nil {
		log.Printf("Error converting DVLA data: %v", err)
		http.Redirect(w, r, "/dashboard?error=Error+processing+vehicle+data", http.StatusSeeOther)
		return
	}

	// Update car record in database
	_, err = database.Queries.UpdateCarDVLAData(context.Background(), db.UpdateCarDVLADataParams{
		ID:                           pgtype.UUID{Bytes: carID, Valid: true},
		Make:                         convertStringPtr(carData.Make),
		Colour:                       convertStringPtr(carData.Colour),
		FuelType:                     convertStringPtr(carData.FuelType),
		EngineCapacity:               convertInt32Ptr(carData.EngineCapacity),
		YearOfManufacture:           convertInt32Ptr(carData.YearOfManufacture),
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
		log.Printf("Error updating car record: %v", err)
		http.Redirect(w, r, "/dashboard?error=Error+updating+car+data", http.StatusSeeOther)
		return
	}

	http.Redirect(w, r, "/dashboard?success=Car+data+refreshed+successfully", http.StatusSeeOther)
}

// Admin car management handlers
func AdminCarsHandler(w http.ResponseWriter, r *http.Request) {
	// Get all cars with user information
	cars, err := database.Queries.GetAllCarsWithUsers(context.Background())
	if err != nil {
		log.Printf("Error fetching all cars: %v", err)
		cars = []db.GetAllCarsWithUsersRow{} // Empty slice if error
	}

	data := models.PageData{
		Title:           "Manage Cars",
		IsAuthenticated: true,
		AllCars:         cars,
		Success:         r.URL.Query().Get("success"),
		Error:           r.URL.Query().Get("error"),
	}

	RenderTemplate(w, r, "admin_cars.html", data)
}

// Helper functions
func renderAddCarError(w http.ResponseWriter, r *http.Request, errorMsg string) {
	data := models.PageData{
		Title:           "Add Car",
		IsAuthenticated: true,
		Error:           errorMsg,
	}
	RenderTemplate(w, r, "add_car.html", data)
}

func convertDVLAResponseToCarData(userID uuid.UUID, dvlaResp *utils.DVLAResponse) (*models.DVLACarData, error) {
	carData := &models.DVLACarData{
		UserID:             userID,
		RegistrationNumber: dvlaResp.RegistrationNumber,
		DVLADataFetchedAt:  time.Now(),
	}

	// Convert string pointers for optional fields
	if dvlaResp.Make != "" {
		carData.Make = &dvlaResp.Make
	}
	if dvlaResp.Colour != "" {
		carData.Colour = &dvlaResp.Colour
	}
	if dvlaResp.FuelType != "" {
		carData.FuelType = &dvlaResp.FuelType
	}
	if dvlaResp.TaxStatus != "" {
		carData.TaxStatus = &dvlaResp.TaxStatus
	}
	if dvlaResp.MOTStatus != "" {
		carData.MOTStatus = &dvlaResp.MOTStatus
	}
	if dvlaResp.EuroStatus != "" {
		carData.EuroStatus = &dvlaResp.EuroStatus
	}
	if dvlaResp.RealDrivingEmissions != "" {
		carData.RealDrivingEmissions = &dvlaResp.RealDrivingEmissions
	}
	if dvlaResp.TypeApproval != "" {
		carData.TypeApproval = &dvlaResp.TypeApproval
	}
	if dvlaResp.Wheelplan != "" {
		carData.Wheelplan = &dvlaResp.Wheelplan
	}
	if dvlaResp.MonthOfFirstRegistration != "" {
		carData.MonthOfFirstRegistration = &dvlaResp.MonthOfFirstRegistration
	}
	if dvlaResp.MonthOfFirstDVLARegistration != "" {
		carData.MonthOfFirstDVLARegistration = &dvlaResp.MonthOfFirstDVLARegistration
	}

	// Convert integer fields
	if dvlaResp.EngineCapacity > 0 {
		val := int32(dvlaResp.EngineCapacity)
		carData.EngineCapacity = &val
	}
	if dvlaResp.YearOfManufacture > 0 {
		val := int32(dvlaResp.YearOfManufacture)
		carData.YearOfManufacture = &val
	}
	if dvlaResp.CO2Emissions > 0 {
		val := int32(dvlaResp.CO2Emissions)
		carData.CO2Emissions = &val
	}
	if dvlaResp.RevenueWeight > 0 {
		val := int32(dvlaResp.RevenueWeight)
		carData.RevenueWeight = &val
	}

	// Convert boolean fields
	carData.AutomatedVehicle = &dvlaResp.AutomatedVehicle
	carData.MarkedForExport = &dvlaResp.MarkedForExport

	// Convert date fields
	if taxDueDate, err := dvlaResp.GetTaxDueDate(); err == nil && taxDueDate != nil {
		carData.TaxDueDate = taxDueDate
	}
	if motExpiryDate, err := dvlaResp.GetMOTExpiryDate(); err == nil && motExpiryDate != nil {
		carData.MOTExpiryDate = motExpiryDate
	}
	if v5cIssued, err := dvlaResp.GetDateOfLastV5CIssued(); err == nil && v5cIssued != nil {
		carData.DateOfLastV5CIssued = v5cIssued
	}
	if artEndDate, err := dvlaResp.GetArtEndDate(); err == nil && artEndDate != nil {
		carData.ArtEndDate = artEndDate
	}

	return carData, nil
}

func convertTimeToDate(t *time.Time) pgtype.Date {
	if t == nil {
		return pgtype.Date{Valid: false}
	}
	return pgtype.Date{Time: *t, Valid: true}
}

func convertStringPtr(s *string) pgtype.Text {
	if s == nil {
		return pgtype.Text{Valid: false}
	}
	return pgtype.Text{String: *s, Valid: true}
}

func convertInt32Ptr(i *int32) pgtype.Int4 {
	if i == nil {
		return pgtype.Int4{Valid: false}
	}
	return pgtype.Int4{Int32: *i, Valid: true}
}

func convertBoolPtr(b *bool) pgtype.Bool {
	if b == nil {
		return pgtype.Bool{Valid: false}
	}
	return pgtype.Bool{Bool: *b, Valid: true}
}
