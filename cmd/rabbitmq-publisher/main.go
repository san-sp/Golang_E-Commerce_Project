/*
1. Connect to RabbitMQ
2. Create channel
3. Ensure exchange exists
4. Publish message
5. Close resources
*/

package main

import "github.com/rabbitmq/amqp091-go"

func main() {
	conn, err := amqp091.Dial("amqp://guest:guest@localhost:5672/")
	if err != nil {
		println("Failed to connect to RabbitMQ")
		return
	}
	defer conn.Close()

	ch, err := conn.Channel()
	if err != nil {
		println("Faile to create RabbitMQ channel")
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

	message := amqp091.Publishing{
		ContentType:  "application/json",
		DeliveryMode: amqp091.Persistent,
		Body:         []byte(`{"payment_id":"pay_123","order_id":"order_123","amount":899900}`),
	}

	err = ch.Publish(
		"ecommerce.events",
		"payment.succeeded",
		false,
		false,
		message,
	)
	if err != nil {
		println("Failed to publish message")
		return
	}

	println("Connected to RabbitMQ")
	println("RabbitMQ channel created")
	println("Exchange declared")
	println("Message published")
}
