package main

import (
	"errors"

	"github.com/rabbitmq/amqp091-go"
	"github.com/san-sp/Golang_E-Commerce_Project/internal/messaging"
)

const maxRetries = 3

func handleMessage(message amqp091.Delivery) (messaging.ProcessingResult, error) {
	println(string(message.Body))

	// return messaging.ProcessingSuccess, nil
	return messaging.ProcessingRetry, errors.New("temporary failure")
}

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

	err = ch.ExchangeDeclare(
		"payment-retry-exchange",
		"direct",
		true,
		false,
		false,
		false,
		nil,
	)
	if err != nil {
		println("Failed to declare retry exchange")
		return
	}

	err = ch.Confirm(false)
	if err != nil {
		println("Failed to enable publisher confirms")
		return
	}

	// confirms := ch.NotifyPublish(make(chan amqp091.Confirmation, 1))

	queue, err := ch.QueueDeclare(
		"payment-events",
		true,
		false,
		false,
		false,
		// nil,
		amqp091.Table{
			"x-dead-letter-exchange":    "payment-dlx",
			"x-dead-letter-routing-key": "payment.dead",
		},
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

	consumer := messaging.NewConsumer(ch)

	publisher, err := messaging.NewPublisher(ch)
	if err != nil {
		println("Failed to create publisher:", err)
		return
	}

	retryHandler := func(message amqp091.Delivery) error {
		retryCount := 0

		if value, ok := message.Headers["retry-count"]; ok {
			retryCount = int(value.(int32))
		}

		if retryCount >= maxRetries {
			return publisher.Publish(
				"payment-dlx",
				"payment.dead",
				amqp091.Publishing{
					ContentType:  message.ContentType,
					DeliveryMode: message.DeliveryMode,
					Body:         message.Body,
					Headers:      message.Headers,
				},
			)
		}

		retryCount++

		headers := amqp091.Table{
			"retry-count": int32(retryCount),
		}

		return publisher.Publish(
			"payment-retry-exchange",
			"payment.retry",
			amqp091.Publishing{
				ContentType:  message.ContentType,
				DeliveryMode: message.DeliveryMode,
				Body:         message.Body,
				Headers:      headers,
			},
		)
	}

	err = consumer.SetQoS(1)
	if err != nil {
		println("Failed to set QoS")
		return
	}

	err = consumer.Start(
		queue.Name,
		handleMessage,
		retryHandler,
	)

	if err != nil {
		println("Consumer stopped:", err)
		return
	}

	println("Connected to RabbitMQ")
	println("RabbitMQ channel created")
	println("Exchange declared")
	println("Payment queue declared")
	println("Payment queue bound to exchange")
}
