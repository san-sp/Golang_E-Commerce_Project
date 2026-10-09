package messaging

import (
	"errors"
	"testing"
	"time"

	"github.com/rabbitmq/amqp091-go"
)

type fakePublisher struct {
	exchange   string
	routingKey string
	message    amqp091.Publishing
	err        error
	calls      int
}

func (f *fakePublisher) Publish(
	exchange string,
	routingKey string,
	message amqp091.Publishing,
) error {
	f.exchange = exchange
	f.routingKey = routingKey
	f.message = message
	f.calls++
	return f.err
}

func TestRetryPolicyHandlePreservesMetadata(t *testing.T) {
	publisher := &fakePublisher{}

	policy := NewRetryPolicy(
		publisher,
		3,
		"retry-exchange",
		"payment.retry",
		"dlq-exchange",
		"payment.dead",
	)

	originalHeaders := amqp091.Table{
		"trace-id":    "trace-123",
		"retry-count": int32(0),
	}

	message := amqp091.Delivery{
		Headers:         originalHeaders,
		ContentType:     "application/json",
		ContentEncoding: "utf-8",
		DeliveryMode:    amqp091.Persistent,
		Priority:        2,
		CorrelationId:   "correlation-123",
		MessageId:       "event-123",
		Timestamp:       time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
		Type:            "PAYMENT_SUCCEEDED",
		AppId:           "ecommerce",
		Body:            []byte(`{"event":"payment"}`),
	}

	if err := policy.Handle(message); err != nil {
		t.Fatalf("Handle() error = %v", err)
	}

	if publisher.calls != 1 {
		t.Fatalf("Publish() calls = %d, want 1", publisher.calls)
	}

	if publisher.exchange != "retry-exchange" {
		t.Errorf("exchange = %q, want retry-exchange", publisher.exchange)
	}

	if publisher.routingKey != "payment.retry" {
		t.Errorf("routing key = %q, want payment.retry", publisher.routingKey)
	}

	if publisher.message.MessageId != message.MessageId {
		t.Errorf(
			"MessageId = %q, want %q",
			publisher.message.MessageId,
			message.MessageId,
		)
	}

	if publisher.message.Headers["trace-id"] != "trace-123" {
		t.Error("existing trace-id header was not preserved")
	}

	if got := publisher.message.Headers["retry-count"]; got != int32(1) {
		t.Errorf("retry-count = %v, want 1", got)
	}

	if originalHeaders["retry-count"] != int32(0) {
		t.Error("Handle() modified the original headers")
	}

	if publisher.message.ContentEncoding != message.ContentEncoding {
		t.Error("ContentEncoding was not preserved")
	}

	if publisher.message.CorrelationId != message.CorrelationId {
		t.Error("CorrelationId was not preserved")
	}

	if publisher.message.Type != message.Type {
		t.Error("Type was not preserved")
	}

	if string(publisher.message.Body) != string(message.Body) {
		t.Error("Body was not preserved")
	}
}

func TestRetryPolicyHandlePublishesToDLQ(t *testing.T) {
	publisher := &fakePublisher{}

	policy := NewRetryPolicy(
		publisher,
		2,
		"retry-exchange",
		"payment.retry",
		"dlq-exchange",
		"payment.dead",
	)

	message := amqp091.Delivery{
		Headers: amqp091.Table{
			"retry-count": int32(2),
			"trace-id":    "trace-456",
		},
		MessageId: "event-456",
		Body:      []byte(`{"event":"payment"}`),
	}

	if err := policy.Handle(message); err != nil {
		t.Fatalf("Handle() error = %v", err)
	}

	if publisher.exchange != "dlq-exchange" {
		t.Errorf("exchange = %q, want dlq-exchange", publisher.exchange)
	}

	if publisher.routingKey != "payment.dead" {
		t.Errorf("routing key = %q, want payment.dead", publisher.routingKey)
	}

	if publisher.message.MessageId != "event-456" {
		t.Errorf("MessageId = %q, want event-456", publisher.message.MessageId)
	}

	if publisher.message.Headers["trace-id"] != "trace-456" {
		t.Error("trace-id header was not preserved in DLQ message")
	}

	if publisher.message.Headers["retry-count"] != int32(2) {
		t.Error("retry-count was not preserved in DLQ message")
	}
}

func TestRetryPolicyHandleReturnsPublishError(t *testing.T) {
	publishErr := errors.New("publish failed")
	publisher := &fakePublisher{err: publishErr}

	policy := NewRetryPolicy(
		publisher,
		3,
		"retry-exchange",
		"payment.retry",
		"dlq-exchange",
		"payment.dead",
	)

	err := policy.Handle(amqp091.Delivery{
		Headers:   amqp091.Table{},
		MessageId: "event-789",
	})

	if !errors.Is(err, publishErr) {
		t.Errorf("Handle() error = %v, want %v", err, publishErr)
	}
}
