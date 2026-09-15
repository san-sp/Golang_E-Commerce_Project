package main

import (
	"github.com/rabbitmq/amqp091-go"
)

func main() {
	conn, err := amqp091.Dial("amqp://guest:guest@localhost:5672/")
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

	queue, err := ch.QueueDeclare(
		"payment-events",
		true,
		false,
		false,
		false,
		nil,
	)
	if err != nil {
		println("Failed to decalre queue")
		return
	}

	err = ch.QueueBind(
		queue.Name,
		"payment.*",
		"ecommerce.events",
		false,
		nil,
	)
	if err != nil {
		println("Failed to bind queue")
		return
	}

	messages, err := ch.Consume(
		queue.Name,
		"",
		false,
		false,
		false,
		false,
		nil,
	)
	if err != nil {
		println("Failed to register consumer")
	}

	for message := range messages {
		println(string(message.Body))
	}

	// message := amqp091.Publishing{
	// 	ContentType:  "application/json",
	// 	DeliveryMode: amqp091.Persistent,
	// 	Body:         []byte(`{"payment_id":"pay_123","order_id":"order_123","amount":"899900}`),
	// }

	// err = ch.Publish(
	// 	"ecommerce.events",
	// 	"payment.succeeded",
	// 	false,
	// 	false,
	// 	message,
	// )
	// if err != nil {
	// 	println("Failed to publish message")
	// }

	println("Connected to RabbitMQ")
	println("RabbitMQ channel created")
	println("Exchange declared")
	println("Payment queue declared")
	println("Payment queue bound to exchange")
	// println("Message published")
}
