package reservation

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/joho/godotenv"

	"github.com/san-sp/Golang_E-Commerce_Project/internal/database"
	"github.com/san-sp/Golang_E-Commerce_Project/internal/database/db"
)

func createReservationTestInventory(
	t *testing.T,
	pool *pgxpool.Pool,
) pgtype.UUID {
	t.Helper()

	ctx := context.Background()

	var productID pgtype.UUID

	err := pool.QueryRow(
		ctx,
		`INSERT INTO products (name, description)
		 VALUES ('Reservation Test Product', 'Test product')
		 RETURNING id`,
	).Scan(&productID)
	if err != nil {
		t.Fatalf("create test product: %v", err)
	}

	var variantID pgtype.UUID

	err = pool.QueryRow(
		ctx,
		`INSERT INTO product_variants (
			product_id,
			sku,
			price
		)
		VALUES (
			$1,
			'reservation-test-' || gen_random_uuid()::text,
			899900
		)
		RETURNING id`,
		productID,
	).Scan(&variantID)
	if err != nil {
		t.Fatalf("create test variant: %v", err)
	}

	_, err = pool.Exec(
		ctx,
		`INSERT INTO inventory (
			variant_id,
			quantity
		)
		VALUES ($1, 10)`,
		variantID,
	)
	if err != nil {
		t.Fatalf("create test inventory: %v", err)
	}

	t.Cleanup(func() {
		_, err := pool.Exec(
			ctx,
			`DELETE FROM reservations
			 WHERE variant_id = $1`,
			variantID,
		)
		if err != nil {
			t.Logf("cleanup reservations failed: %v", err)
		}

		_, err = pool.Exec(
			ctx,
			`DELETE FROM inventory
			 WHERE variant_id = $1`,
			variantID,
		)
		if err != nil {
			t.Logf("cleanup inventory failed: %v", err)
		}

		_, err = pool.Exec(
			ctx,
			`DELETE FROM product_variants
			 WHERE id = $1`,
			variantID,
		)
		if err != nil {
			t.Logf("cleanup product variant failed: %v", err)
		}

		_, err = pool.Exec(
			ctx,
			`DELETE FROM products
			 WHERE id = $1`,
			productID,
		)
		if err != nil {
			t.Logf("cleanup product failed: %v", err)
		}
	})

	return variantID
}

func TestCreateReservation(t *testing.T) {
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

	service := NewService(
		pool,
		queries,
		15*time.Minute,
	)

	variantID := createReservationTestInventory(t, pool)

	reservation, err := service.CreateReservation(
		ctx,
		variantID,
		1,
	)
	if err != nil {
		t.Fatalf("create reservation: %v", err)
	}

	if reservation.Status != "ACTIVE" {
		t.Fatalf(
			"expected reservation status ACTIVE, got %s",
			reservation.Status,
		)
	}

	expectedExpiration := time.Now().Add(15 * time.Minute)

	if reservation.ExpiresAt.Time.Before(expectedExpiration.Add(-5*time.Second)) ||
		reservation.ExpiresAt.Time.After(expectedExpiration.Add(5*time.Second)) {
		t.Fatalf(
			"expected expiration to be approximately 15 minutes from now, got %s",
			reservation.ExpiresAt.Time,
		)
	}

	if reservation.Quantity != 1 {
		t.Fatalf(
			"expected reservation quantity 1, got %d",
			reservation.Quantity,
		)
	}

	t.Logf("created reservation: %s", reservation.ID)

	_, err = service.CreateReservation(
		ctx,
		variantID,
		999999,
	)

	if err == nil {
		t.Fatal("expected insufficient stock error, got nil")
	}

	if !errors.Is(err, ErrInsufficientStock) {
		t.Fatalf(
			"expected ErrInsufficientStock, got %v",
			err,
		)
	}

	t.Logf("expected reservation failure: %v", err)

}

func TestExpireReservation(t *testing.T) {
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

	variantID := createReservationTestInventory(t, pool)

	expiredReservation, err := queries.CreateReservation(ctx, db.CreateReservationParams{
		VariantID: variantID,
		Quantity:  1,
		ExpiresAt: pgtype.Timestamptz{
			Time:  time.Now().Add(-1 * time.Minute),
			Valid: true,
		},
	})
	if err != nil {
		t.Fatalf("create expired reservation: %v", err)
	}

	expired, err := queries.ExpireReservation(ctx, expiredReservation.ID)
	if err != nil {
		t.Fatalf("expire reservation: %v", err)
	}

	if expired.Status != "EXPIRED" {
		t.Fatalf(
			"expected reservation status EXPIRED, got %s",
			expired.Status,
		)
	}

	futureReservation, err := queries.CreateReservation(ctx, db.CreateReservationParams{
		VariantID: variantID,
		Quantity:  1,
		ExpiresAt: pgtype.Timestamptz{
			Time:  time.Now().Add(10 * time.Minute),
			Valid: true,
		},
	})
	if err != nil {
		t.Fatalf("create future reservation: %v", err)
	}

	_, err = queries.ExpireReservation(ctx, futureReservation.ID)
	if err == nil {
		t.Fatal("expected future reservation to remain ACTIVE, got nil")
	}
}
