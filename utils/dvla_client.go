package utils

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	"github.com/google/uuid"
)

const (
	DVLABaseURL = "https://driver-vehicle-licensing.api.gov.uk/vehicle-enquiry/v1/vehicles"
)

type DVLARequest struct {
	RegistrationNumber string `json:"registrationNumber"`
}

type DVLAResponse struct {
	RegistrationNumber             string    `json:"registrationNumber"`
	TaxStatus                      string    `json:"taxStatus"`
	TaxDueDate                     string    `json:"taxDueDate"`
	ArtEndDate                     string    `json:"artEndDate"`
	MOTStatus                      string    `json:"motStatus"`
	MOTExpiryDate                  string    `json:"motExpiryDate"`
	Make                           string    `json:"make"`
	MonthOfFirstRegistration       string    `json:"monthOfFirstRegistration"`
	MonthOfFirstDVLARegistration   string    `json:"monthOfFirstDvlaRegistration"`
	YearOfManufacture              int       `json:"yearOfManufacture"`
	EngineCapacity                 int       `json:"engineCapacity"`
	CO2Emissions                   int       `json:"co2Emissions"`
	FuelType                       string    `json:"fuelType"`
	MarkedForExport                bool      `json:"markedForExport"`
	Colour                         string    `json:"colour"`
	TypeApproval                   string    `json:"typeApproval"`
	RevenueWeight                  int       `json:"revenueWeight"`
	DateOfLastV5CIssued            string    `json:"dateOfLastV5CIssued"`
	Wheelplan                      string    `json:"wheelplan"`
	EuroStatus                     string    `json:"euroStatus"`
	RealDrivingEmissions           string    `json:"realDrivingEmissions"`
	AutomatedVehicle               bool      `json:"automatedVehicle"`
}

type DVLAClient struct {
	APIKey     string
	HTTPClient *http.Client
}

func NewDVLAClient() *DVLAClient {
	apiKey := os.Getenv("DVLA_API_KEY")
	if apiKey == "" {
		panic("DVLA_API_KEY environment variable is required")
	}

	return &DVLAClient{
		APIKey: apiKey,
		HTTPClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

func (c *DVLAClient) GetVehicleData(registrationNumber string) (*DVLAResponse, error) {
	request := DVLARequest{
		RegistrationNumber: registrationNumber,
	}

	jsonData, err := json.Marshal(request)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequest("POST", DVLABaseURL, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Generate correlation ID for request tracking
	correlationID := uuid.New().String()

	// Set required headers
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("x-api-key", c.APIKey)
	req.Header.Set("X-Correlation-Id", correlationID)

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to make request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("DVLA API returned error %d: %s", resp.StatusCode, string(body))
	}

	var dvlaResp DVLAResponse
	if err := json.Unmarshal(body, &dvlaResp); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	return &dvlaResp, nil
}

func ParseDate(dateStr string) (*time.Time, error) {
	if dateStr == "" {
		return nil, nil
	}

	// Try different date formats that DVLA might use
	formats := []string{
		"2006-01-02",     // YYYY-MM-DD
		"2006-01",        // YYYY-MM
		"02-01-2006",     // DD-MM-YYYY
		"01-2006",        // MM-YYYY
	}

	for _, format := range formats {
		if t, err := time.Parse(format, dateStr); err == nil {
			return &t, nil
		}
	}

	return nil, fmt.Errorf("unable to parse date: %s", dateStr)
}

func (resp *DVLAResponse) GetTaxDueDate() (*time.Time, error) {
	return ParseDate(resp.TaxDueDate)
}

func (resp *DVLAResponse) GetMOTExpiryDate() (*time.Time, error) {
	return ParseDate(resp.MOTExpiryDate)
}

func (resp *DVLAResponse) GetDateOfLastV5CIssued() (*time.Time, error) {
	return ParseDate(resp.DateOfLastV5CIssued)
}

func (resp *DVLAResponse) GetArtEndDate() (*time.Time, error) {
	return ParseDate(resp.ArtEndDate)
}