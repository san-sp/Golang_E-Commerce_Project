package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/joho/godotenv"

	"github.com/san-sp/Golang_E-Commerce_Project/internal/database"
	"github.com/san-sp/Golang_E-Commerce_Project/internal/database/db"
	"github.com/san-sp/Golang_E-Commerce_Project/internal/reservation"
)

func main() {
	if err := godotenv.Load(".env"); err != nil {
		fmt.Println("no .env file found, using environment variables")
	}

	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		fmt.Println("DATABASE_URL is not set")
		os.Exit(1)
	}

	ctx, stop := signal.NotifyContext(
		context.Background(),
		os.Interrupt,
		syscall.SIGTERM,
	)
	defer stop()

	pool, err := database.NewPostgres(ctx, databaseURL)
	if err != nil {
		fmt.Printf("connect to PostgreSQL: %v\n", err)
		os.Exit(1)
	}
	defer pool.Close()

	queries := db.New(pool)

	worker := reservation.NewWorker(
		queries,
		5*time.Second,
		100,
	)

	fmt.Println("reservation expiration worker started")

	err = worker.Run(ctx)
	if err != nil {
		fmt.Printf("reservation expiration worker stopped: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("reservation expiration worker stopped")
}
