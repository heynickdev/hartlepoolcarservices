package database

import (
	"database/sql"
	"log"
	"os"

	_ "github.com/jackc/pgx/v5/stdlib" // PostgreSQL driver
	"github.com/pressly/goose/v3"
)

// RunMigrations connects to the database and applies any pending migrations.
func RunMigrations() error {
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		log.Fatal("DATABASE_URL environment variable is not set")
	}

	db, err := sql.Open("pgx", dbURL)
	if err != nil {
		return err
	}
	defer db.Close()

	// It's good practice to set a dialect
	if err := goose.SetDialect("postgres"); err != nil {
		return err
	}

	// Run migrations
	if err := goose.Up(db, "migrations"); err != nil {
		return err
	}

	log.Println("Database migrations applied successfully.")
	return nil
}
