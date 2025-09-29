package models

import (
	"hcs-full/database/db"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

type Claims struct {
	UserID  uuid.UUID `json:"user_id"`
	Email   string    `json:"email"`
	Role   string    `json:"role"`
	jwt.RegisteredClaims
}

type PageData struct {
	Title                 string
	IsAuthenticated       bool
	User                  *db.User
	UserCars              []db.Car
	AllCars               []db.GetAllCarsWithUsersRow
	AllUsers              []db.User
	SelectedCar           *db.Car
	VehicleOwner          *db.User
	CarAppointments       []db.Appointment
	AllAppointments       []db.GetAllAppointmentsRow
	UserAppointments      []db.GetAppointmentsForUserRow
	CarNextAppointments   map[uuid.UUID]*db.Appointment
	Success               string
	Error                 string
	ErrorMessage          string
	Token                 string
	TotalCars             int
	TotalAppointments     int
	TotalUsers            int
	AcceptedAppointments  int
	PendingAppointments   int
	CancelledAppointments int
	Calendar              CalendarData
	ChartData             ChartData
	CarsByUser            map[uuid.UUID]UserCarSummary
	RoleStats             map[string]int
	MetaDescription       string
	CanonicalURL          string
}

type CalendarData struct {
	Month      string
	Year       int
	DaysOfWeek []string
	Days       []CalendarDay
	MonthIndex int
}

type CalendarDay struct {
	Number       int
	IsToday      bool
	Appointments []AppointmentInfo
}

type AppointmentInfo struct {
	Title    string `json:"title"`
	Time     string `json:"time"`
	UserName string `json:"userName,omitempty"`
}

type ChartData struct {
	Labels    []string `json:"labels"`
	Confirmed []int    `json:"confirmed"`
	Pending   []int    `json:"pending"`
	Cancelled []int    `json:"cancelled"`
}

type Message struct {
	UserID uuid.UUID   `json:"userId"`
	Type   string      `json:"type"`
	Data   interface{} `json:"data"`
}

type UserCarSummary struct {
	User  db.User
	Count int
}

type CreateAppointmentRequest struct {
	CarID       string `json:"carId" form:"carId"`
	DateTime    string `json:"datetime" form:"datetime"`
	Title       string `json:"title" form:"title"`
	Description string `json:"description" form:"description"`
}

// Car-related form and request models
type AddCarRequest struct {
	RegistrationNumber string `json:"registrationNumber" form:"registrationNumber"`
}

type CarFormData struct {
	RegistrationNumber string
	Error              string
	Success            string
}

// DVLA response mapping for creating car records
type DVLACarData struct {
	UserID                       uuid.UUID
	RegistrationNumber           string
	Make                         *string
	Colour                       *string
	FuelType                     *string
	EngineCapacity               *int32
	YearOfManufacture            *int32
	MonthOfFirstRegistration     *string
	MonthOfFirstDVLARegistration *string
	TaxStatus                    *string
	TaxDueDate                   *time.Time
	MOTStatus                    *string
	MOTExpiryDate                *time.Time
	CO2Emissions                 *int32
	EuroStatus                   *string
	RealDrivingEmissions         *string
	RevenueWeight                *int32
	TypeApproval                 *string
	Wheelplan                    *string
	AutomatedVehicle             *bool
	MarkedForExport              *bool
	DateOfLastV5CIssued          *time.Time
	ArtEndDate                   *time.Time
	DVLADataFetchedAt            time.Time
}
