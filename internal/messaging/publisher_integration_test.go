package messaging

import (
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/rabbitmq/amqp091-go"
)

func TestPublisherIntegration(t *testing.T) {
	url := os.Getenv("RABBITMQ_URL")
	if url == "" {
		t.Skip("set RABBITMQ_URL to run RabbitMQ integration tests")
	}

	conn, err := amqp091.Dial(url)
	if err != nil {
		t.Fatalf("connect to RabbitMQ: %v", err)
	}
	defer conn.Close()

	channel, err := conn.Channel()
	if err != nil {
		t.Fatalf("open RabbitMQ channel: %v", err)
	}
	defer channel.Close()

	exchange := "publisher-test-" + uuid.NewString()

	if err := channel.ExchangeDeclare(
		exchange,
		"direct",
		false,
		true,
		false,
		false,
		nil,
	); err != nil {
		t.Fatalf("declare test exchange: %v", err)
	}

	queue, err := channel.QueueDeclare(
		"",
		false,
		true,
		true,
		false,
		nil,
	)
	if err != nil {
		t.Fatalf("declare test queue: %v", err)
	}

	if err := channel.QueueBind(
		queue.Name,
		"test.routed",
		exchange,
		false,
		nil,
	); err != nil {
		t.Fatalf("bind test queue: %v", err)
	}

	publisher, err := NewPublisher(channel)
	if err != nil {
		t.Fatalf("create publisher: %v", err)
	}

	t.Run("routed message succeeds", func(t *testing.T) {
		err := publisher.Publish(
			exchange,
			"test.routed",
			amqp091.Publishing{
				ContentType:  "text/plain",
				DeliveryMode: amqp091.Persistent,
				MessageId:    uuid.NewString(),
				Body:         []byte("hello RabbitMQ"),
			},
		)
		if err != nil {
			t.Fatalf("Publish() error = %v", err)
		}
	})

	t.Run("unroutable mandatory message fails", func(t *testing.T) {
		err := publisher.Publish(
			exchange,
			fmt.Sprintf("unrouted.%s", uuid.NewString()),
			amqp091.Publishing{
				ContentType:  "text/plain",
				DeliveryMode: amqp091.Persistent,
				MessageId:    uuid.NewString(),
				Body:         []byte("this message has no route"),
			},
		)
		if err == nil {
			t.Fatal("Publish() error = nil, want unroutable-message error")
		}
	})

	_ = time.Second // Remove this line and the time import if unused.
}
