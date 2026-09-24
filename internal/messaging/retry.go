package messaging

import (
	"fmt"

	"github.com/rabbitmq/amqp091-go"
)

type RetryPolicy struct {
	publisher       *Publisher
	maxRetries      int
	retryExchange   string
	retryRoutingKey string
	dqlExchange     string
	dlqRoutingKey   string
}

func NewRetryPolicy(
	publisher *Publisher,
	maxRetries int,
	retryExchange string,
	retryRoutingKey string,
	dlqExchange string,
	dlqRoutingKey string,
) *RetryPolicy {
	return &RetryPolicy{
		publisher:       publisher,
		maxRetries:      maxRetries,
		retryExchange:   retryExchange,
		retryRoutingKey: retryRoutingKey,
		dqlExchange:     dlqExchange,
		dlqRoutingKey:   dlqRoutingKey,
	}
}

func (r *RetryPolicy) Handle(message amqp091.Delivery) error {
	retryCount := 0

	if value, ok := message.Headers["retry-count"]; ok {
		count, ok := value.(int32)

		if !ok {
			return fmt.Errorf("ivalid retry-count header type: %T", value)
		}

		retryCount = int(count)
	}

	if retryCount >= r.maxRetries {
		return r.publisher.Publish(
			r.dqlExchange,
			r.dlqRoutingKey,
			amqp091.Publishing{
				ContentType:  message.ContentType,
				DeliveryMode: message.DeliveryMode,
				Headers:      message.Headers,
				Body:         message.Body,
			},
		)
	}

	retryCount++

	headers := amqp091.Table{
		"retry-count": int32(retryCount),
	}

	return r.publisher.Publish(
		r.retryExchange,
		r.retryRoutingKey,
		amqp091.Publishing{
			ContentType:  message.ContentType,
			DeliveryMode: message.DeliveryMode,
			Headers:      headers,
			Body:         message.Body,
		},
	)
}
