package main

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/joho/godotenv"
	"github.com/rabbitmq/amqp091-go"

	"github.com/san-sp/Golang_E-Commerce_Project/internal/messaging"
	"github.com/san-sp/Golang_E-Commerce_Project/internal/payment"
)

func main() {
	err := godotenv.Load()
	if err != nil {
		fmt.Println("Warning: .env file not loaded")
	}

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

		fmt.Println("Processing payment succeeded event:")
		fmt.Println("Payment ID:", event.PaymentID)
		fmt.Println("Reservation ID:", event.ReservationID)
		fmt.Println("Amount:", event.Amount)
		fmt.Println("Currency:", event.Currency)

		return messaging.ProcessingSuccess, nil
	}

	err = consumer.Start(
		"payment-events",
		handler,
		nil,
	)
	if err != nil {
		fmt.Println("Consumer stopped:", err)
	}
}
