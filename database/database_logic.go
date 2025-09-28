package database

import (
	"context"
	"log"
	"os"

	"hcs-full/database/db"
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

