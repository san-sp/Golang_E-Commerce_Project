package payment

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/joho/godotenv"

	"github.com/san-sp/Golang_E-Commerce_Project/internal/database"
	"github.com/san-sp/Golang_E-Commerce_Project/internal/database/db"
	"github.com/san-sp/Golang_E-Commerce_Project/internal/reservation"
)

func TestProcessPaymentSucceeded(t *testing.T) {
	err := godotenv.Load("../../.env")
	if err != nil {
		t.Fatalf("load .env: %v", err)
	}

	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		t.Fatal("DATABASE_URL is not set")
	}

	ctx := context.Background()

	pool, err := database.NewPostgres(ctx, databaseURL)
	if err != nil {
		t.Fatalf("connect to PostgreSQL: %v", err)
	}

	t.Cleanup(func() {
		pool.Close()
	})

	queries := db.New(pool)
	provider := NewMockProvider()

	service := NewService(
		pool,
		queries,
		provider,
	)

	consumerService := NewConsumerService(
		pool,
		queries,
	)

	reservationService := reservation.NewService(
		pool,
		queries,
	)

	variantID := pgtype.UUID{
		Bytes: [16]byte{
			0xc5, 0x17, 0x4f, 0x98,
			0x5b, 0xa2, 0x45, 0x3e,
			0x92, 0xfb, 0x26, 0x6e,
			0x81, 0x8f, 0xbd, 0x92,
		},
		Valid: true,
	}

	expiresAt := pgtype.Timestamptz{
		Time:  time.Now().Add(10 * time.Minute),
		Valid: true,
	}

	createdReservation, err := reservationService.CreateReservation(
		ctx,
		variantID,
		1,
		expiresAt,
	)
	if err != nil {
		t.Fatalf("create test reservation: %v", err)
	}

	payment, err := service.CreatePayment(
		ctx,
		createdReservation.ID,
		"mock",
		899900,
		"INR",
	)
	if err != nil {
		t.Fatalf("create payment: %v", err)
	}

	eventID := pgtype.UUID{
		Bytes: uuid.New(),
		Valid: true,
	}

	consumerName := "payment-consumer"

	t.Cleanup(func() {
		_, err := pool.Exec(
			ctx,
			"DELETE FROM payment_processing WHERE payment_id = $1",
			payment.ID,
		)
		if err != nil {
			t.Logf(
				"cleanup payment processing failed: %v",
				err,
			)
		}

		_, err = pool.Exec(
			ctx,
			`DELETE FROM consumer_events
         WHERE consumer_name = $1
           AND event_id = $2`,
			consumerName,
			eventID,
		)
		if err != nil {
			t.Logf(
				"cleanup consumer event failed: %v",
				err,
			)
		}

		_, err = pool.Exec(
			ctx,
			"DELETE FROM payments WHERE id = $1",
			payment.ID,
		)
		if err != nil {
			t.Logf("cleanup payment failed: %v", err)
		}

		_, err = pool.Exec(
			ctx,
			"DELETE FROM reservations WHERE id = $1",
			createdReservation.ID,
		)
		if err != nil {
			t.Logf("cleanup reservation failed: %v", err)
		}
	})

	err = consumerService.ProcessPaymentSucceeded(
		ctx,
		consumerName,
		eventID,
		payment.ID,
	)
	if err != nil {
		t.Fatalf("process payment succeeded event: %v", err)
	}

	var consumerEventCount int

	err = pool.QueryRow(
		ctx,
		`SELECT COUNT(*)
		 FROM consumer_events
		 WHERE consumer_name = $1
		   AND event_id = $2`,
		consumerName,
		eventID,
	).Scan(&consumerEventCount)

	if err != nil {
		t.Fatalf("count consumer events: %v", err)
	}

	if consumerEventCount != 1 {
		t.Fatalf(
			"expected 1 consumer event, got %d",
			consumerEventCount,
		)
	}

	var processingCount int

	err = pool.QueryRow(
		ctx,
		`SELECT COUNT(*)
		 FROM payment_processing
		 WHERE payment_id = $1`,
		payment.ID,
	).Scan(&processingCount)

	if err != nil {
		t.Fatalf("count payment processing rows: %v", err)
	}

	if processingCount != 1 {
		t.Fatalf(
			"expected 1 payment processing row, got %d",
			processingCount,
		)
	}
}

func TestProcessPaymentSucceededDuplicate(t *testing.T) {
	err := godotenv.Load("../../.env")
	if err != nil {
		t.Fatalf("load .env: %v", err)
	}

	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		t.Fatal("DATABASE_URL is not set")
	}

	ctx := context.Background()

	pool, err := database.NewPostgres(ctx, databaseURL)
	if err != nil {
		t.Fatalf("connect to PostgreSQL: %v", err)
	}

	t.Cleanup(func() {
		pool.Close()
	})

	queries := db.New(pool)
	provider := NewMockProvider()

	service := NewService(
		pool,
		queries,
		provider,
	)

	consumerService := NewConsumerService(
		pool,
		queries,
	)

	reservationService := reservation.NewService(
		pool,
		queries,
	)

	variantID := pgtype.UUID{
		Bytes: [16]byte{
			0xc5, 0x17, 0x4f, 0x98,
			0x5b, 0xa2, 0x45, 0x3e,
			0x92, 0xfb, 0x26, 0x6e,
			0x81, 0x8f, 0xbd, 0x92,
		},
		Valid: true,
	}

	expiresAt := pgtype.Timestamptz{
		Time:  time.Now().Add(10 * time.Minute),
		Valid: true,
	}

	createdReservation, err := reservationService.CreateReservation(
		ctx,
		variantID,
		1,
		expiresAt,
	)
	if err != nil {
		t.Fatalf("create test reservation: %v", err)
	}

	payment, err := service.CreatePayment(
		ctx,
		createdReservation.ID,
		"mock",
		899900,
		"INR",
	)
	if err != nil {
		t.Fatalf("create payment: %v", err)
	}

	eventID := pgtype.UUID{
		Bytes: uuid.New(),
		Valid: true,
	}

	consumerName := "payment-consumer"

	t.Cleanup(func() {
		_, err := pool.Exec(
			ctx,
			"DELETE FROM payment_processing WHERE payment_id = $1",
			payment.ID,
		)
		if err != nil {
			t.Logf(
				"cleanup payment processing failed: %v",
				err,
			)
		}

		_, err = pool.Exec(
			ctx,
			`DELETE FROM consumer_events
         WHERE consumer_name = $1
           AND event_id = $2`,
			consumerName,
			eventID,
		)
		if err != nil {
			t.Logf(
				"cleanup consumer event failed: %v",
				err,
			)
		}

		_, err = pool.Exec(
			ctx,
			"DELETE FROM payments WHERE id = $1",
			payment.ID,
		)
		if err != nil {
			t.Logf("cleanup payment failed: %v", err)
		}

		_, err = pool.Exec(
			ctx,
			"DELETE FROM reservations WHERE id = $1",
			createdReservation.ID,
		)
		if err != nil {
			t.Logf("cleanup reservation failed: %v", err)
		}
	})

	err = consumerService.ProcessPaymentSucceeded(
		ctx,
		consumerName,
		eventID,
		payment.ID,
	)
	if err != nil {
		t.Fatalf("first processing failed: %v", err)
	}

	err = consumerService.ProcessPaymentSucceeded(
		ctx,
		consumerName,
		eventID,
		payment.ID,
	)
	if !errors.Is(err, ErrConsumerEventAlreadyProcessed) {
		t.Fatalf(
			"expected ErrConsumerEventAlreadyProcessed, got %v",
			err,
		)
	}

	var consumerEventCount int

	err = pool.QueryRow(
		ctx,
		`SELECT COUNT(*)
		 FROM consumer_events
		 WHERE consumer_name = $1
		   AND event_id = $2`,
		consumerName,
		eventID,
	).Scan(&consumerEventCount)

	if err != nil {
		t.Fatalf("count consumer events: %v", err)
	}

	if consumerEventCount != 1 {
		t.Fatalf(
			"expected 1 consumer event after duplicate, got %d",
			consumerEventCount,
		)
	}

	var processingCount int

	err = pool.QueryRow(
		ctx,
		`SELECT COUNT(*)
		 FROM payment_processing
		 WHERE payment_id = $1`,
		payment.ID,
	).Scan(&processingCount)

	if err != nil {
		t.Fatalf("count payment processing rows: %v", err)
	}

	if processingCount != 1 {
		t.Fatalf(
			"expected 1 payment processing row after duplicate, got %d",
			processingCount,
		)
	}
}

func TestProcessPaymentSucceededRollback(t *testing.T) {
	err := godotenv.Load("../../.env")
	if err != nil {
		t.Fatalf("load .env: %v", err)
	}

	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		t.Fatal("DATABASE_URL is not set")
	}

	ctx := context.Background()

	pool, err := database.NewPostgres(ctx, databaseURL)
	if err != nil {
		t.Fatalf("connect to PostgreSQL: %v", err)
	}

	t.Cleanup(func() {
		pool.Close()
	})

	queries := db.New(pool)

	consumerService := NewConsumerService(
		pool,
		queries,
	)

	eventID := pgtype.UUID{
		Bytes: uuid.New(),
		Valid: true,
	}

	invalidPaymentID := pgtype.UUID{
		Bytes: [16]byte{
			0x44, 0x44, 0x44, 0x44,
			0x44, 0x44, 0x44, 0x44,
			0x44, 0x44, 0x44, 0x44,
			0x44, 0x44, 0x44, 0x44,
		},
		Valid: true,
	}

	consumerName := "payment-consumer"

	err = consumerService.ProcessPaymentSucceeded(
		ctx,
		consumerName,
		eventID,
		invalidPaymentID,
	)

	if err == nil {
		t.Fatal("expected payment processing to fail")
	}

	var consumerEventCount int

	err = pool.QueryRow(
		ctx,
		`SELECT COUNT(*)
		 FROM consumer_events
		 WHERE consumer_name = $1
		   AND event_id = $2`,
		consumerName,
		eventID,
	).Scan(&consumerEventCount)

	if err != nil {
		t.Fatalf("count consumer events: %v", err)
	}

	if consumerEventCount != 0 {
		t.Fatalf(
			"expected consumer event to rollback, got %d rows",
			consumerEventCount,
		)
	}

	var processingCount int

	err = pool.QueryRow(
		ctx,
		`SELECT COUNT(*)
		 FROM payment_processing
		 WHERE payment_id = $1`,
		invalidPaymentID,
	).Scan(&processingCount)

	if err != nil {
		t.Fatalf("count payment processing rows: %v", err)
	}

	if processingCount != 0 {
		t.Fatalf(
			"expected payment processing to rollback, got %d rows",
			processingCount,
		)
	}
}
