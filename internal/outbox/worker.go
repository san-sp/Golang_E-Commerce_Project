package outbox

import (
	"context"
	"fmt"
	"time"

	"github.com/rabbitmq/amqp091-go"
	"github.com/san-sp/Golang_E-Commerce_Project/internal/database/db"
	"github.com/san-sp/Golang_E-Commerce_Project/internal/messaging"
)

type Worker struct {
	queries   *db.Queries
	publisher *messaging.Publisher
	interval  time.Duration
}

func NewWorker(
	queries *db.Queries,
	publisher *messaging.Publisher,
	interval time.Duration,
) *Worker {
	return &Worker{
		queries:   queries,
		publisher: publisher,
		interval:  interval,
	}
}

func (w *Worker) Run(ctx context.Context) error {
	ticker := time.NewTicker(w.interval)
	defer ticker.Stop()

	for {
		if err := w.process(ctx); err != nil {
			fmt.Println("Outbox processing failed:", err)
		}

		select {
		case <-ticker.C:
		case <-ctx.Done():
			fmt.Println("Shutting down outbox worker...")
			return nil
		}
	}
}

func (w *Worker) process(ctx context.Context) error {
	err := w.queries.RecoverStaleOutboxEvents(ctx)
	if err != nil {
		return fmt.Errorf("recover stale outbox events: %w", err)
	}

	fmt.Println("Recovered stale outbox events")

	events, err := w.queries.ClaimPendingOutboxEvents(ctx)
	if err != nil {
		return fmt.Errorf("claim pending outbox events: %w", err)
	}

	fmt.Println("Claimed outbox events:", len(events))

	for _, event := range events {
		fmt.Printf(
			"Processing event: %s | Type: %s\n",
			event.ID,
			event.EventType,
		)

		routingKey, err := routingKeyForEvent(event.EventType)
		if err != nil {
			fmt.Printf(
				"Failed to determine routing key for %s: %v\n",
				event.ID,
				err,
			)
			continue
		}

		err = w.queries.MarkOutboxPublishAttempt(ctx, event.ID)
		if err != nil {
			fmt.Printf(
				"Failed to record publish attempt for %s: %v\n",
				event.ID,
				err,
			)
			continue
		}

		err = w.publisher.Publish(
			"ecommerce.events",
			routingKey,
			amqp091.Publishing{
				ContentType:  "application/json",
				DeliveryMode: amqp091.Persistent,
				Body:         event.Payload,
			},
		)
		if err != nil {
			fmt.Printf(
				"Failed to publish event %s: %v\n",
				event.ID,
				err,
			)
			continue
		}

		err = w.queries.MarkOutboxEventPublished(ctx, event.ID)
		if err != nil {
			fmt.Printf(
				"Failed to mark event %s as published: %v\n",
				event.ID,
				err,
			)
			continue
		}

		fmt.Println("Event published successfully")
	}

	return nil
}

func routingKeyForEvent(eventType string) (string, error) {
	switch eventType {
	case "PAYMENT_SUCCEEDED":
		return "payment.succeeded", nil
	case "ORDER_CREATED":
		return "order.created", nil
	case "RESERVATION_CONFIRMED":
		return "reservation.confirmed", nil
	default:
		return "", fmt.Errorf("unknown event type: %s", eventType)
	}
}
