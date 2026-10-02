package messaging

import (
	"fmt"

	"github.com/rabbitmq/amqp091-go"
)

func SetupTopology(channel *amqp091.Channel) error {
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
		return fmt.Errorf("declare ecommerce.events exchange: %w", err)
	}

	_, err = channel.QueueDeclare(
		"payment-events",
		true,
		false,
		false,
		false,
		nil,
	)
	if err != nil {
		return fmt.Errorf("declare payment-events queue: %w", err)
	}

	err = channel.QueueBind(
		"payment-events",
		"payment.*",
		"ecommerce.events",
		false,
		nil,
	)
	if err != nil {
		return fmt.Errorf("bind payment-events queue: %w", err)
	}

	_, err = channel.QueueDeclare(
		"order-events",
		true,
		false,
		false,
		false,
		nil,
	)
	if err != nil {
		return fmt.Errorf("declare order-events queue: %w", err)
	}

	err = channel.QueueBind(
		"order-events",
		"order.#",
		"ecommerce.events",
		false,
		nil,
	)
	if err != nil {
		return fmt.Errorf("bind order-events queue: %w", err)
	}

	return nil
}
