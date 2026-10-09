package messaging

import (
	"fmt"

	"github.com/rabbitmq/amqp091-go"
)

type messagePublisher interface {
	Publish(
		exchange string,
		routingKey string,
		message amqp091.Publishing,
	) error
}

type RetryPolicy struct {
	publisher       messagePublisher
	maxRetries      int
	retryExchange   string
	retryRoutingKey string
	dlqExchange     string
	dlqRoutingKey   string
}

func NewRetryPolicy(
	publisher messagePublisher,
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
		dlqExchange:     dlqExchange,
		dlqRoutingKey:   dlqRoutingKey,
	}
}

func (r *RetryPolicy) Handle(message amqp091.Delivery) error {
	retryCount := 0

	if value, ok := message.Headers["retry-count"]; ok {
		count, ok := value.(int32)
		if !ok {
			return fmt.Errorf(
				"invalid retry-count header type: %T",
				value,
			)
		}

		retryCount = int(count)
	}

	headers := make(amqp091.Table, len(message.Headers)+1)
	for key, value := range message.Headers {
		headers[key] = value
	}

	publishing := amqp091.Publishing{
		Headers:         headers,
		ContentType:     message.ContentType,
		ContentEncoding: message.ContentEncoding,
		DeliveryMode:    message.DeliveryMode,
		Priority:        message.Priority,
		CorrelationId:   message.CorrelationId,
		ReplyTo:         message.ReplyTo,
		Expiration:      message.Expiration,
		MessageId:       message.MessageId,
		Timestamp:       message.Timestamp,
		Type:            message.Type,
		UserId:          message.UserId,
		AppId:           message.AppId,
		Body:            message.Body,
	}

	if retryCount >= r.maxRetries {
		return r.publisher.Publish(
			r.dlqExchange,
			r.dlqRoutingKey,
			publishing,
		)
	}

	retryCount++
	headers["retry-count"] = int32(retryCount)

	return r.publisher.Publish(
		r.retryExchange,
		r.retryRoutingKey,
		publishing,
	)
}
