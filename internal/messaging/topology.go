package messaging

import (
	"fmt"

	"github.com/rabbitmq/amqp091-go"
)

func SetupTopology(channel *amqp091.Channel) error {
	// --------------------------------------------------
	// Main event exchange
	// --------------------------------------------------

	err := channel.ExchangeDeclare(
		"ecommerce.events",
		"topic",
		true,
		false,
		false,
		false,
		nil,
	)
	if err != nil {
		return fmt.Errorf(
			"declare ecommerce.events exchange: %w",
			err,
		)
	}

	// --------------------------------------------------
	// Payment DLQ topology
	// --------------------------------------------------

	err = channel.ExchangeDeclare(
		"payment-dlx",
		"direct",
		true,
		false,
		false,
		false,
		nil,
	)
	if err != nil {
		return fmt.Errorf(
			"declare payment-dlx exchange: %w",
			err,
		)
	}

	_, err = channel.QueueDeclare(
		"payment-dlq",
		true,
		false,
		false,
		false,
		nil,
	)
	if err != nil {
		return fmt.Errorf(
			"declare payment-dlq queue: %w",
			err,
		)
	}

	err = channel.QueueBind(
		"payment-dlq",
		"payment.dead",
		"payment-dlx",
		false,
		nil,
	)
	if err != nil {
		return fmt.Errorf(
			"bind payment-dlq queue: %w",
			err,
		)
	}

	// --------------------------------------------------
	// Payment retry topology
	// --------------------------------------------------

	err = channel.ExchangeDeclare(
		"payment-retry-exchange",
		"direct",
		true,
		false,
		false,
		false,
		nil,
	)
	if err != nil {
		return fmt.Errorf(
			"declare payment-retry-exchange: %w",
			err,
		)
	}

	_, err = channel.QueueDeclare(
		"payment-retry",
		true,
		false,
		false,
		false,
		amqp091.Table{
			"x-message-ttl":             int32(5000),
			"x-dead-letter-exchange":    "retry-dlx",
			"x-dead-letter-routing-key": "payment.retry",
		},
	)
	if err != nil {
		return fmt.Errorf(
			"declare payment-retry queue: %w",
			err,
		)
	}

	err = channel.QueueBind(
		"payment-retry",
		"payment.retry",
		"payment-retry-exchange",
		false,
		nil,
	)
	if err != nil {
		return fmt.Errorf(
			"bind payment-retry queue: %w",
			err,
		)
	}

	// --------------------------------------------------
	// Retry dead-letter exchange
	// --------------------------------------------------

	err = channel.ExchangeDeclare(
		"retry-dlx",
		"direct",
		true,
		false,
		false,
		false,
		nil,
	)
	if err != nil {
		return fmt.Errorf(
			"declare retry-dlx exchange: %w",
			err,
		)
	}

	_, err = channel.QueueDeclare(
		"payment-events",
		true,
		false,
		false,
		false,
		amqp091.Table{
			"x-dead-letter-exchange":    "payment-dlx",
			"x-dead-letter-routing-key": "payment.dead",
		},
	)
	if err != nil {
		return fmt.Errorf(
			"declare payment-events queue: %w",
			err,
		)
	}

	err = channel.QueueBind(
		"payment-events",
		"payment.retry",
		"retry-dlx",
		false,
		nil,
	)
	if err != nil {
		return fmt.Errorf(
			"bind retry-dlx to payment-events: %w",
			err,
		)
	}

	// --------------------------------------------------
	// Payment events queue
	// --------------------------------------------------

	_, err = channel.QueueDeclare(
		"payment-events",
		true,
		false,
		false,
		false,
		amqp091.Table{
			"x-dead-letter-exchange":    "payment-dlx",
			"x-dead-letter-routing-key": "payment.dead",
		},
	)
	if err != nil {
		return fmt.Errorf(
			"declare payment-events queue: %w",
			err,
		)
	}

	err = channel.QueueBind(
		"payment-events",
		"payment.*",
		"ecommerce.events",
		false,
		nil,
	)
	if err != nil {
		return fmt.Errorf(
			"bind payment-events queue: %w",
			err,
		)
	}

	// --------------------------------------------------
	// Order events queue
	// --------------------------------------------------

	_, err = channel.QueueDeclare(
		"order-events",
		true,
		false,
		false,
		false,
		nil,
	)
	if err != nil {
		return fmt.Errorf(
			"declare order-events queue: %w",
			err,
		)
	}

	err = channel.QueueBind(
		"order-events",
		"order.#",
		"ecommerce.events",
		false,
		nil,
	)
	if err != nil {
		return fmt.Errorf(
			"bind order-events queue: %w",
			err,
		)
	}

	return nil
}
