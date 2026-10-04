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

func TestCreatePayment(t *testing.T) {
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

	reservationService := reservation.NewService(
		pool,
		queries,
		15*time.Minute,
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

	createdReservation, err := reservationService.CreateReservation(
		ctx,
		variantID,
		1,
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

	t.Cleanup(func() {
		_, err := pool.Exec(
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

	if payment.Status != "PENDING" {
		t.Fatalf(
			"expected payment status PENDING, got %s",
			payment.Status,
		)
	}

	if !payment.ProviderPaymentID.Valid {
		t.Fatal("expected provider payment ID to be set")
	}

	if !payment.ProviderPaymentID.Valid {
		t.Fatal("expected provider payment ID to be valid")
	}

	if payment.ProviderPaymentID.String == "" {
		t.Fatal("expected provider payment ID to be non-empty")
	}

	if payment.Amount != 899900 {
		t.Fatalf(
			"expected amount 899900, got %d",
			payment.Amount,
		)
	}

	if payment.Currency != "INR" {
		t.Fatalf(
			"expected currency INR, got %s",
			payment.Currency,
		)
	}

	t.Logf(
		"created payment: %s at %s",
		payment.ID,
		time.Now().Format(time.RFC3339),
	)
}

func TestCreatePaymentProviderFailure(t *testing.T) {
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
	provider.SetFailure(true)

	service := NewService(
		pool,
		queries,
		provider,
	)

	reservationService := reservation.NewService(
		pool,
		queries,
		15*time.Minute,
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

	createdReservation, err := reservationService.CreateReservation(
		ctx,
		variantID,
		1,
	)
	if err != nil {
		t.Fatalf("create test reservation: %v", err)
	}

	_, err = service.CreatePayment(
		ctx,
		createdReservation.ID,
		"mock",
		899900,
		"INR",
	)

	if err == nil {
		t.Fatal("expected payment provider failure")
	}

	t.Logf("payment creation failed as expected: %v", err)

	var status string
	var providerPaymentID pgtype.Text

	err = pool.QueryRow(
		ctx,
		`SELECT status, provider_payment_id
	 FROM payments
	 WHERE reservation_id = $1`,
		createdReservation.ID,
	).Scan(&status, &providerPaymentID)

	if err != nil {
		t.Fatalf("check payment after provider failure: %v", err)
	}

	t.Logf(
		"payment after provider failure: status=%s provider_payment_id_valid=%v",
		status,
		providerPaymentID.Valid,
	)

	if status != "FAILED" {
		t.Fatalf(
			"expected payment status FAILED, got %s",
			status,
		)
	}

	if providerPaymentID.Valid {
		t.Fatal("expected provider payment ID to remain NULL")
	}

	t.Cleanup(func() {
		_, err := pool.Exec(
			ctx,
			"DELETE FROM payments WHERE reservation_id = $1",
			createdReservation.ID,
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
}

func TestCreatePaymentProviderUnknown(t *testing.T) {
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
	provider.SetUnknown()

	service := NewService(
		pool,
		queries,
		provider,
	)

	reservationService := reservation.NewService(
		pool,
		queries,
		15*time.Minute,
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

	createdReservation, err := reservationService.CreateReservation(
		ctx,
		variantID,
		1,
	)
	if err != nil {
		t.Fatalf("create test reservation: %v", err)
	}

	_, err = service.CreatePayment(
		ctx,
		createdReservation.ID,
		"mock",
		899900,
		"INR",
	)

	if err == nil {
		t.Fatal("expected unknown provider outcome")
	}

	var status string
	var providerPaymentID pgtype.Text

	err = pool.QueryRow(
		ctx,
		`SELECT status, provider_payment_id
	 FROM payments
	 WHERE reservation_id = $1`,
		createdReservation.ID,
	).Scan(&status, &providerPaymentID)

	if err != nil {
		t.Fatalf("check payment after unknown provider outcome: %v", err)
	}

	if status != "PENDING" {
		t.Fatalf(
			"expected payment status PENDING, got %s",
			status,
		)
	}

	if providerPaymentID.Valid {
		t.Fatal("expected provider payment ID to remain NULL")
	}

	t.Logf(
		"payment after unknown provider outcome: status=%s provider_payment_id_valid=%v",
		status,
		providerPaymentID.Valid,
	)

	t.Cleanup(func() {
		_, err := pool.Exec(
			ctx,
			"DELETE FROM payments WHERE reservation_id = $1",
			createdReservation.ID,
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
}

func TestMarkPaymentSucceeded(t *testing.T) {
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

	reservationService := reservation.NewService(
		pool,
		queries,
		15*time.Minute,
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

	createdReservation, err := reservationService.CreateReservation(
		ctx,
		variantID,
		1,
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

	updatedPayment, err := service.MarkPaymentSucceeded(
		ctx,
		payment.ID,
	)
	if err != nil {
		t.Fatalf("mark payment succeeded: %v", err)
	}

	if updatedPayment.Status != "SUCCEEDED" {
		t.Fatalf(
			"expected payment status SUCCEEDED, got %s",
			updatedPayment.Status,
		)
	}

	if !updatedPayment.ProviderPaymentID.Valid {
		t.Fatal("expected provider payment ID to be present")
	}

	if !updatedPayment.ProviderPaymentID.Valid {
		t.Fatal("expected provider payment ID to be valid")
	}

	if updatedPayment.ProviderPaymentID.String == "" {
		t.Fatal("expected provider payment ID to be non-empty")
	}

	_, err = service.MarkPaymentSucceeded(
		ctx,
		payment.ID,
	)

	if err == nil {
		t.Fatal("expected second payment success transition to fail")
	}

	t.Logf(
		"second success transition failed as expected: %v",
		err,
	)

	t.Cleanup(func() {
		_, err := pool.Exec(
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

	if payment.Amount != 899900 {
		t.Fatalf(
			"expected amount 899900, got %d",
			payment.Amount,
		)
	}

	if payment.Currency != "INR" {
		t.Fatalf(
			"expected currency INR, got %s",
			payment.Currency,
		)
	}

	t.Logf(
		"created payment: %s at %s",
		payment.ID,
		time.Now().Format(time.RFC3339),
	)
}
