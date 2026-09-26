package main

import (
	"context"
	"fmt"
	"os"

	"github.com/joho/godotenv"
	"github.com/rabbitmq/amqp091-go"

	"github.com/san-sp/Golang_E-Commerce_Project/internal/database"
	"github.com/san-sp/Golang_E-Commerce_Project/internal/database/db"
	"github.com/san-sp/Golang_E-Commerce_Project/internal/messaging"
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

	ctx := context.Background()

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

	err = queries.RecoverStaleOutboxEvents(ctx)
	if err != nil {
		fmt.Println("Failed to recover stale outbox events:", err)
		return
	}

	fmt.Println("Recovered stale outbox events")

	events, err := queries.ClaimPendingOutboxEvents(ctx)
	if err != nil {
		fmt.Println("Failed to fetch pending outbox events:", err)
		return
	}

	fmt.Println("Claimed outbox events:", len(events))

	for _, event := range events {
		fmt.Printf(
			"Processing event: %s | Type: %s\n",
			event.ID,
			event.EventType,
		)

		err := queries.MarkOutboxPublishAttempt(ctx, event.ID)
		if err != nil {
			fmt.Println("Failed to record publish attempt:", err)
			continue
		}

		err = publisher.Publish(
			"ecommerce.events",
			"payment.succeeded",
			amqp091.Publishing{
				ContentType:  "application/json",
				DeliveryMode: amqp091.Persistent,
				Body:         event.Payload,
			},
		)
		if err != nil {
			fmt.Println("Failed to publish event:", err)
			continue
		}

		err = queries.MarkOutboxEventPublished(ctx, event.ID)
		if err != nil {
			fmt.Println("Failed to mark event as published:", err)
			continue
		}
		fmt.Println("Event published successfully")
	}
}
