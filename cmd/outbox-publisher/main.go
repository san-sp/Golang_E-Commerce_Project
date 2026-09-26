package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"time"

	"github.com/joho/godotenv"

	"github.com/san-sp/Golang_E-Commerce_Project/internal/database"
	"github.com/san-sp/Golang_E-Commerce_Project/internal/database/db"
	"github.com/san-sp/Golang_E-Commerce_Project/internal/messaging"
	"github.com/san-sp/Golang_E-Commerce_Project/internal/outbox"
)

func main() {

	err := godotenv.Load()
	if err != nil {
		fmt.Println("Warning: .env file not loaded")
	}

	databaseURL := os.Getenv("DATABASE_URL")

	if databaseURL == "" {
		fmt.Println("DATABASE_URL is not set")
		return
	}

	ctx, stop := signal.NotifyContext(
		context.Background(),
		os.Interrupt,
	)
	defer stop()

	pool, err := database.NewPostgres(ctx, databaseURL)
	if err != nil {
		fmt.Println("Failed to connect to PostgreSQL:", err)
		return
	}
	defer pool.Close()

	fmt.Println("Connected to PostgreSQL")

	rabbitMQURL := os.Getenv("RABBITMQ_URL")

	if rabbitMQURL == "" {
		fmt.Println("RABBITMQ_URL is not set")
		return
	}

	rabbitConn, err := messaging.NewRabbitMQ(rabbitMQURL)
	if err != nil {
		fmt.Println("Failed to connect to RabbitMQ:", err)
		return
	}
	defer rabbitConn.Close()

	rabbitChannel, err := rabbitConn.Channel()
	if err != nil {
		fmt.Println("Failed to create RabbitMQ channel:", err)
		return
	}
	defer rabbitChannel.Close()

	publisher, err := messaging.NewPublisher(rabbitChannel)
	if err != nil {
		fmt.Println("Failed to create RabbitMQ publisher:", err)
		return
	}

	if publisher == nil {
		fmt.Println("Publisher was not created")
		return
	}

	fmt.Println("Connected to RabbitMQ")

	queries := db.New(pool)

	worker := outbox.NewWorker(
		queries,
		publisher,
		5*time.Second,
	)

	if err := worker.Run(ctx); err != nil {
		fmt.Println("Outbox worker failed:", err)
	}
}
