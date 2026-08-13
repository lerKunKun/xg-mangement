package main

import (
	"context"
	"log"
	"os"
	"time"

	"github.com/xg-management/platform/backend/migrations"
)

func main() {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		databaseURL = "postgres://xg:xg@localhost:5432/xg?sslmode=disable"
	}
	if err := migrations.Run(ctx, databaseURL); err != nil {
		log.Fatal(err)
	}
	log.Print("database migrations applied")
}
