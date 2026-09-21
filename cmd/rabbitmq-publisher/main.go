package main

import (
	"github.com/rabbitmq/amqp091-go"
	"github.com/san-sp/Golang_E-Commerce_Project/internal/messaging"
)

func main() {
	conn, err := messaging.NewRabbitMQ("amqp://guest:guest@localhost:5672/")
	if err != nil {
		println("Failed to connect to RabbitMQ")
		return
	}
	defer conn.Close()

	ch, err := conn.Channel()
	if err != nil {
		println("Failed to create RabbitMQ channel")
		return
	}
	defer ch.Close()

	publisher, err := messaging.NewPublisher(ch)
	if err != nil {
		println("Failed to create publisher")
		return
	}

	err = ch.ExchangeDeclare(
		"ecommerce.events",
		"topic",
		true,
		false,
		false,
		false,
		nil,
	)
	if err != nil {
		println("Failed to declare exchange")
		return
	}

	message := amqp091.Publishing{
		ContentType:  "application/json",
		DeliveryMode: amqp091.Persistent,
		Body:         []byte(`{"payment_id":"pay_123","order_id":"order_123","amount":899900}`),
	}

	err = publisher.Publish(
		"ecommerce.events",
		"payment.succeeded",
		message,
	)
	if err != nil {
		println("Failed to publish message")
		return
	}

	println("Message published")
}
