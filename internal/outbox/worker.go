package outbox

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/rabbitmq/amqp091-go"
	"github.com/san-sp/Golang_E-Commerce_Project/internal/database/db"
	"github.com/san-sp/Golang_E-Commerce_Project/internal/messaging"
)

type Worker struct {
	queries          *db.Queries
	publisher        *messaging.Publisher
	interval         time.Duration
	cleanupInterval  time.Duration
	retention        time.Duration
	cleanupBatchSize int32
	batchSize        int32
}

func NewWorker(
	queries *db.Queries,
	publisher *messaging.Publisher,
	interval time.Duration,
) *Worker {
	return &Worker{
		queries:          queries,
		publisher:        publisher,
		interval:         interval,
		cleanupInterval:  time.Hour,
		retention:        7 * 24 * time.Hour,
		cleanupBatchSize: 500,
		batchSize:        10,
	}
}

func (w *Worker) cleanup(ctx context.Context) error {
	cutoff := time.Now().Add(-w.retention)

	deleted, err := w.queries.DeletePublishedOutboxEventsBefore(
		ctx,
		db.DeletePublishedOutboxEventsBeforeParams{
			PublishedAt: pgtype.Timestamptz{
				Time:  cutoff,
				Valid: true,
			},
			Limit: w.cleanupBatchSize,
		},
	)
	if err != nil {
		return fmt.Errorf("delete old published outbox events: %w", err)
	}

	if deleted > 0 {
		fmt.Println("Deleted old published outbox events:", deleted)
	}

	return nil
}

func (w *Worker) Run(ctx context.Context) error {
	ticker := time.NewTicker(w.interval)
	defer ticker.Stop()

	cleanupTicker := time.NewTicker(w.cleanupInterval)
	defer cleanupTicker.Stop()

	if err := w.cleanup(ctx); err != nil {
		fmt.Println("Initial outbox cleanup failed:", err)
	}

	for {
		if err := w.process(ctx); err != nil {
			fmt.Println("Outbox processing failed:", err)
		}

		select {
		case <-ticker.C:
			continue

		case <-cleanupTicker.C:
			if err := w.cleanup(ctx); err != nil {
				fmt.Println("Outbox cleanup failed:", err)
			}

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

	events, err := w.queries.ClaimPendingOutboxEvents(ctx, w.batchSize)
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

		rowsAffected, err := w.queries.MarkOutboxPublishAttempt(
			ctx,
			db.MarkOutboxPublishAttemptParams{
				ID:              event.ID,
				ProcessingToken: event.ProcessingToken,
			},
		)
		if err != nil {
			fmt.Printf(
				"Failed to record publish attempt for %s: %v\n",
				event.ID,
				err,
			)
			continue
		}

		if rowsAffected != 1 {
			fmt.Printf(
				"Skipping event %s: claim ownership was lost\n",
				event.ID,
			)
			continue
		}

		err = w.publisher.Publish(
			"ecommerce.events",
			routingKey,
			amqp091.Publishing{
				ContentType:  "application/json",
				DeliveryMode: amqp091.Persistent,
				MessageId:    event.ID.String(),
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

		rowsAffected, err = w.queries.MarkOutboxEventPublished(
			ctx,
			db.MarkOutboxEventPublishedParams{
				ID:              event.ID,
				ProcessingToken: event.ProcessingToken,
			},
		)
		if err != nil {
			fmt.Printf(
				"Failed to mark event %s as published: %v\n",
				event.ID,
				err,
			)
			continue
		}

		if rowsAffected != 1 {
			fmt.Printf(
				"Published event %s, but claim ownership was lost before marking it published\n",
				event.ID,
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
