package messaging

import (
	"github.com/rabbitmq/amqp091-go"
)

type ProcessingResult int

const (
	ProcessingSuccess ProcessingResult = iota
	ProcessingRetry
	ProcessingReject
)

// type MessageHandler func(amqp091.Delivery) error
type MessageHandler func(amqp091.Delivery) (ProcessingResult, error)

type RetryHandler func(amqp091.Delivery) error

type Consumer struct {
	channel *amqp091.Channel
}

func NewConsumer(channel *amqp091.Channel) *Consumer {
	return &Consumer{
		channel: channel,
	}
}

// func (ch *amqp091.Channel) Qos(prefetchCount int, prefetchSize int, global bool) error
func (c *Consumer) SetQoS(prefetchCount int) error {
	return c.channel.Qos(
		prefetchCount,
		0,
		false,
	)
}

// func (ch *amqp091.Channel) Consume(queue string, consumer string, autoAck bool, exclusive bool, noLocal bool, noWait bool, args amqp091.Table) (<-chan amqp091.Delivery, error)
func (c *Consumer) Consume(queueName string) (<-chan amqp091.Delivery, error) {
	return c.channel.Consume(
		queueName,
		"",
		false,
		false,
		false,
		false,
		nil,
	)
}

func (c *Consumer) Start(
	queueName string,
	handler MessageHandler,
	retryHandler RetryHandler,
) error {
	messages, err := c.Consume(queueName)

	if err != nil {
		return err
	}

	for message := range messages {
		result, err := handler(message)

		if err != nil {
			println("Message processing error:", err.Error())
		}

		switch result {
		case ProcessingSuccess:
			err = message.Ack(false)
			if err != nil {
				return err
			}

		case ProcessingRetry:
			err = retryHandler(message)

			if err != nil {
				return err
			}

			err = message.Ack(false)

			if err != nil {
				return err
			}

		case ProcessingReject:
			err = message.Nack(false, false)

			if err != nil {
				return err
			}
		}

		if err != nil {
			return err
		}
	}

	return nil
}
