package messaging

import (
	"errors"
	"fmt"
	"time"

	"github.com/rabbitmq/amqp091-go"
)

type Publisher struct {
	channel     *amqp091.Channel
	confirms    chan amqp091.Confirmation
	closeNotify chan *amqp091.Error
}

func NewPublisher(channel *amqp091.Channel) (*Publisher, error) {
	err := channel.Confirm(false)
	if err != nil {
		return nil, err
	}

	confirms := channel.NotifyPublish(make(chan amqp091.Confirmation, 1))
	closeNotify := make(chan *amqp091.Error, 1)
	channel.NotifyClose(closeNotify)

	return &Publisher{
		channel:     channel,
		confirms:    confirms,
		closeNotify: closeNotify,
	}, nil
}

func (p *Publisher) Publish(
	exchange string,
	routingKey string,
	message amqp091.Publishing,
) error {
	err := p.channel.Publish(
		exchange,
		routingKey,
		false,
		false,
		message,
	)
	if err != nil {
		return fmt.Errorf("publish message: %w", err)
	}

	select {
	case confirmation, ok := <-p.confirms:
		if !ok {
			return errors.New("RabbitMQ confirmation channel closed")
		}

		if !confirmation.Ack {
			return errors.New("RabbitMQ rejected the message")
		}
	case <-time.After(5 * time.Second):
		return errors.New("Timed out for RabbitMQ confirmation")
	}

	return nil
}
