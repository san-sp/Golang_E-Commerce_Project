package payment

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/joho/godotenv"

	"github.com/san-sp/Golang_E-Commerce_Project/internal/database"
	"github.com/san-sp/Golang_E-Commerce_Project/internal/database/db"
	"github.com/san-sp/Golang_E-Commerce_Project/internal/reservation"
)

func TestRecordPaymentEventDuplicate(t *testing.T) {
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

	reservationService := reservation.NewService(pool, queries)

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

	eventID := "test_event_123"

	_, err = queries.RecordPaymentEvent(
		ctx,
		db.RecordPaymentEventParams{
			EventID:   eventID,
			PaymentID: payment.ID,
			EventType: "payment.succeeded",
		},
	)

	if err != nil {
		t.Fatalf("first event insert failed: %v", err)
	}

	// Send the exact same event again.
	_, err = queries.RecordPaymentEvent(
		ctx,
		db.RecordPaymentEventParams{
			EventID:   eventID,
			PaymentID: payment.ID,
			EventType: "payment.succeeded",
		},
	)

	if err == nil {
		t.Fatal("expected duplicate event to fail")
	}

	t.Logf("duplicate event rejected as expected: %v", err)

	_, err = pool.Exec(
		ctx,
		"DELETE FROM payment_events WHERE payment_id = $1",
		payment.ID,
	)
	if err != nil {
		t.Logf("cleanup payment events failed: %v", err)
	}

	_, err = pool.Exec(
		ctx,
		"DELETE FROM payments WHERE id = $1",
		payment.ID,
	)
	if err != nil {
		t.Logf("cleanup payment failed: %v", err)
	}

	t.Cleanup(func() {
		_, err := pool.Exec(
			ctx,
			"DELETE FROM payments WHERE id = $1",
			payment.ID,
		)
		if err != nil {
			t.Logf("cleanup payment failed: %v", err)
		}
	})

}
