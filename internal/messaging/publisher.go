package messaging

import (
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/rabbitmq/amqp091-go"
)

const publishConfirmationTimeout = 5 * time.Second

type Publisher struct {
	channel     *amqp091.Channel
	confirms    chan amqp091.Confirmation
	returns     chan amqp091.Return
	closeNotify chan *amqp091.Error

	mu     sync.Mutex
	failed error
}

func NewPublisher(channel *amqp091.Channel) (*Publisher, error) {
	if err := channel.Confirm(false); err != nil {
		return nil, fmt.Errorf("enable publisher confirms: %w", err)
	}

	confirms := channel.NotifyPublish(make(chan amqp091.Confirmation, 1))
	returns := channel.NotifyReturn(make(chan amqp091.Return, 1))
	closeNotify := make(chan *amqp091.Error, 1)
	channel.NotifyClose(closeNotify)

	return &Publisher{
		channel:     channel,
		confirms:    confirms,
		returns:     returns,
		closeNotify: closeNotify,
	}, nil
}

func (p *Publisher) Publish(
	exchange string,
	routingKey string,
	message amqp091.Publishing,
) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.failed != nil {
		return fmt.Errorf("publisher is unusable: %w", p.failed)
	}

	if err := p.channel.Publish(
		exchange,
		routingKey,
		true, // mandatory: return if the message cannot be routed
		false,
		message,
	); err != nil {
		return fmt.Errorf("publish message: %w", err)
	}

	timer := time.NewTimer(publishConfirmationTimeout)
	defer timer.Stop()

	select {
	case confirmation, ok := <-p.confirms:
		if !ok {
			return p.invalidate(errors.New(
				"RabbitMQ confirmation channel closed",
			))
		}

		// RabbitMQ sends an unroutable return before its publisher
		// confirmation. Check for that return even if select chose
		// the confirmation case.
		select {
		case returned, ok := <-p.returns:
			if !ok {
				return p.invalidate(errors.New(
					"RabbitMQ return channel closed",
				))
			}
			return fmt.Errorf(
				"RabbitMQ returned unroutable message: exchange=%s routing_key=%s reply_code=%d reply_text=%s",
				returned.Exchange,
				returned.RoutingKey,
				returned.ReplyCode,
				returned.ReplyText,
			)
		default:
		}

		if !confirmation.Ack {
			return errors.New("RabbitMQ rejected the message")
		}

		return nil

	case returned, ok := <-p.returns:
		if !ok {
			return p.invalidate(errors.New(
				"RabbitMQ return channel closed",
			))
		}

		// Consume this message's confirmation before allowing
		// another publish, so it cannot be mistaken for a later one.
		select {
		case confirmation, ok := <-p.confirms:
			if !ok {
				return p.invalidate(errors.New(
					"RabbitMQ confirmation channel closed after return",
				))
			}
			if !confirmation.Ack {
				return fmt.Errorf(
					"RabbitMQ returned unroutable message and rejected it: exchange=%s routing_key=%s reply_code=%d reply_text=%s",
					returned.Exchange,
					returned.RoutingKey,
					returned.ReplyCode,
					returned.ReplyText,
				)
			}
		case <-p.closeNotify:
			return p.invalidate(errors.New(
				"RabbitMQ channel closed after returning a message",
			))
		case <-timer.C:
			return p.invalidate(errors.New(
				"timed out waiting for confirmation after returned message",
			))
		}

		return fmt.Errorf(
			"RabbitMQ returned unroutable message: exchange=%s routing_key=%s reply_code=%d reply_text=%s",
			returned.Exchange,
			returned.RoutingKey,
			returned.ReplyCode,
			returned.ReplyText,
		)

	case <-p.closeNotify:
		return p.invalidate(errors.New(
			"RabbitMQ channel closed while waiting for confirmation",
		))

	case <-timer.C:
		return p.invalidate(errors.New(
			"timed out waiting for RabbitMQ confirmation",
		))
	}
}

func (p *Publisher) invalidate(err error) error {
	if p.failed == nil {
		p.failed = err
	}
	return p.failed
}
