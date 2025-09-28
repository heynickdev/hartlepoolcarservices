package database

import (
	"context"
	"log"
	"os"

	"hcs-full/database/db"
	"hcs-full/utils"

	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	pool    *pgxpool.Pool
	Queries *db.Queries
)

func ConnectDB() {
	var err error
	dbUrl := os.Getenv("DATABASE_URL")
	if dbUrl == "" {
		log.Fatal("DATABASE_URL environment variable is not set")
	}

	pool, err = pgxpool.New(context.Background(), dbUrl)
	if err != nil {
		log.Fatalf("Unable to connect to database: %v\n", err)
	}

	Queries = db.New(pool)
	log.Println("Successfully connected to the database.")
}

func CloseDB() {
	if pool != nil {
		pool.Close()
	}
}

func ActivateTestUser() {
	test, err := Queries.GetUserByEmail(context.Background(), "nickgirling76@gmail.com")
	if err != nil {
		log.Printf("Error getting test user: %v", err)
	}
	err = Queries.VerifyUserEmail(context.Background(), test.ID)
	if err != nil {
		log.Printf("Error verifying test user: %v", err)
	}
}

func SeedAdminUser() error {

	adminEmail := os.Getenv("ADMIN_EMAIL")
	if adminEmail == "" {
		log.Println("ADMIN_EMAIL not set, skipping admin seed.")
		return nil
	}

	adminPassword := os.Getenv("ADMIN_PASSWORD")
	if adminPassword == "" {
		log.Println("ADMIN_PASSWORD not set, skipping admin seed.")
		return nil
	}

	_, err := Queries.GetUserByEmail(context.Background(), adminEmail)
	if err == nil {
		log.Println("Admin user already exists.")
		return nil
	}

	log.Println("Admin user not found, creating one...")

	hashedPassword, err := utils.HashPassword(adminPassword)
	if err != nil {
		return err
	}

	adminParams := db.CreateUserParams{
		Name:         "Admin User",
		Email:        adminEmail,
		PasswordHash: hashedPassword,
		Phone:        "00000000000",
		IsAdmin:      true,
	}
	_, err = Queries.CreateUser(context.Background(), adminParams)
	admin, err := Queries.GetUserByEmail(context.Background(), adminEmail)
	if err != nil {
		return err
	}
	err = Queries.VerifyUserEmail(context.Background(), admin.ID)
	if err != nil {
		return err
	} else {
		log.Println("Admin user created.")
	}
	return err
}
