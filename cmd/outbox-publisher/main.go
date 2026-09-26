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

	err = processOutbox(ctx, queries, publisher)
	if err != nil {
		fmt.Println("Outbox processing failed:", err)
		return
	}
}

func processOutbox(
	ctx context.Context,
	queries *db.Queries,
	publisher *messaging.Publisher,
) error {
	err := queries.RecoverStaleOutboxEvents(ctx)
	if err != nil {
		return fmt.Errorf("recover stale outbox events: %w", err)
	}

	fmt.Println("Recovered stale outbox events")

	events, err := queries.ClaimPendingOutboxEvents(ctx)
	if err != nil {
		return fmt.Errorf("claim pending outbox events: %w", err)
	}

	fmt.Println("Claimed outbox events:", len(events))

	for _, event := range events {
		fmt.Printf(
			"Processing event: %s | Type: %s\n",
			event.ID,
			event.EventType,
		)

		routingKey, err := routingKeyForEvent(event.EventType)
		if err != nil {
			fmt.Printf(
				"Failed to determine routing key for %s: %v\n",
				event.ID,
				err,
			)
			continue
		}

		err = queries.MarkOutboxPublishAttempt(ctx, event.ID)
		if err != nil {
			fmt.Printf(
				"Failed to record publish attempt for %s: %v\n",
				event.ID,
				err,
			)
			continue
		}

		err = publisher.Publish(
			"ecommerce.events",
			routingKey,
			amqp091.Publishing{
				ContentType:  "application/json",
				DeliveryMode: amqp091.Persistent,
				Body:         event.Payload,
			},
		)
		if err != nil {
			fmt.Printf(
				"Failed to publish event %s: %v\n",
				event.ID,
				err,
			)
			continue
		}

		err = queries.MarkOutboxEventPublished(ctx, event.ID)
		if err != nil {
			fmt.Printf(
				"Failed to mark event %s as published: %v\n",
				event.ID,
				err,
			)
			continue
		}

		fmt.Println("Event published successfully")
	}

	return nil
}

func routingKeyForEvent(eventType string) (string, error) {
	switch eventType {
	case "PAYMENT_SUCCEEDED":
		return "payment.succeeded", nil
	case "ORDER_CREATED":
		return "order.created", nil
	case "RESERVATION_CONFIRMED":
		return "reservation.confirmed", nil
	default:
		return "", fmt.Errorf("unknown event type: %s", eventType)
	}
}
