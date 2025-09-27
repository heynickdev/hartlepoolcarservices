package database

import (
	"database/sql"
	"log"
	"os"

	_ "github.com/jackc/pgx/v5/stdlib" // PostgreSQL driver for goose
	"github.com/pressly/goose/v3"
)

// Migrate runs all pending database migrations.
func Migrate() {
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		log.Fatal("DATABASE_URL environment variable is not set")
	}

	db, err := sql.Open("pgx", dbURL)
	if err != nil {
		log.Fatalf("goose: failed to open DB: %v\n", err)
	}
	defer db.Close()

	if err := goose.SetDialect("postgres"); err != nil {
		log.Fatalf("goose: failed to set dialect: %v\n", err)
	}

	log.Println("Applying database migrations...")
	if err := goose.Up(db, "migrations"); err != nil {
		log.Fatalf("goose: up failed: %v", err)
	}
	log.Println("Database migrations applied successfully.")
}

