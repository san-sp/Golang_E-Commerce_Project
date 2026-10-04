package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/joho/godotenv"
	"github.com/rabbitmq/amqp091-go"

	"github.com/san-sp/Golang_E-Commerce_Project/internal/database"
	"github.com/san-sp/Golang_E-Commerce_Project/internal/database/db"
	"github.com/san-sp/Golang_E-Commerce_Project/internal/messaging"
	"github.com/san-sp/Golang_E-Commerce_Project/internal/payment"
)

func main() {
	err := godotenv.Load()
	if err != nil {
		fmt.Println("Warning: .env file not loaded")
	}

	rabbitMQURL := os.Getenv("RABBITMQ_URL")
	databaseURL := os.Getenv("DATABASE_URL")

	if rabbitMQURL == "" {
		fmt.Println("RABBITMQ_URL is not set")
		return
	}

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

	queries := db.New(pool)

	consumerService := payment.NewConsumerService(
		pool,
		queries,
	)

	rabbitConn, err := messaging.NewRabbitMQ(rabbitMQURL)
	if err != nil {
		fmt.Println("Failed to connect to RabbitMQ:", err)
		return
	}
	defer rabbitConn.Close()

	fmt.Println("Connected to RabbitMQ")

	rabbitChannel, err := rabbitConn.Channel()
	if err != nil {
		fmt.Println("Failed to create RabbitMQ channel:", err)
		return
	}
	defer rabbitChannel.Close()

	err = messaging.SetupTopology(rabbitChannel)
	if err != nil {
		fmt.Println("Failed to setup RabbitMQ topology:", err)
		return
	}

	fmt.Println("RabbitMQ topology ready")

	consumer := messaging.NewConsumer(rabbitChannel)

	err = consumer.SetQoS(1)
	if err != nil {
		fmt.Println("Failed to set consumer QoS:", err)
		return
	}

	fmt.Println("Consumer QoS configured")

	publisher, err := messaging.NewPublisher(rabbitChannel)
	if err != nil {
		fmt.Println("Failed to create RabbitMQ publisher:", err)
		return
	}

	retryPolicy := messaging.NewRetryPolicy(
		publisher,
		3,
		"payment-retry-exchange",
		"payment.retry",
		"payment-dlx",
		"payment.dead",
	)

	handler := func(message amqp091.Delivery) (
		messaging.ProcessingResult,
		error,
	) {
		var event payment.PaymentSucceededEvent

		err := json.Unmarshal(message.Body, &event)
		if err != nil {
			return messaging.ProcessingReject, fmt.Errorf(
				"decode payment succeeded event: %w",
				err,
			)
		}

		if message.MessageId == "" {
			return messaging.ProcessingReject, fmt.Errorf(
				"message is missing MessageId",
			)
		}

		eventID, err := uuid.Parse(message.MessageId)
		if err != nil {
			return messaging.ProcessingReject, fmt.Errorf(
				"invalid MessageId: %w",
				err,
			)
		}

		eventIDType := dbUUID(eventID)

		fmt.Println("Processing payment succeeded event:")
		fmt.Println("Event ID:", message.MessageId)
		fmt.Println("Payment ID:", event.PaymentID)
		fmt.Println("Order ID:", event.OrderID)
		fmt.Println("Amount:", event.Amount)
		fmt.Println("Currency:", event.Currency)

		err = consumerService.ProcessPaymentSucceeded(
			ctx,
			"payment-consumer",
			eventIDType,
			event.PaymentID,
		)
		if err != nil {
			if errors.Is(
				err,
				payment.ErrConsumerEventAlreadyProcessed,
			) {
				fmt.Println("Event already processed:", message.MessageId)

				return messaging.ProcessingSuccess, nil
			}

			return messaging.ProcessingRetry, fmt.Errorf(
				"process payment succeeded event: %w",
				err,
			)
		}

		fmt.Println("Payment event processed successfully")

		return messaging.ProcessingSuccess, nil
	}

	err = consumer.Start(
		"payment-events",
		handler,
		retryPolicy.Handle,
	)
	if err != nil {
		fmt.Println("Consumer stopped:", err)
	}
}

func dbUUID(value uuid.UUID) pgtype.UUID {
	return pgtype.UUID{
		Bytes: value,
		Valid: true,
	}
}
