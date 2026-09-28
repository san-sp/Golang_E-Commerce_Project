package reservation

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/joho/godotenv"

	"github.com/san-sp/Golang_E-Commerce_Project/internal/database"
	"github.com/san-sp/Golang_E-Commerce_Project/internal/database/db"
)

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

	service := NewService(pool, queries)

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

	reservation, err := service.CreateReservation(
		ctx,
		variantID,
		1,
		expiresAt,
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
		expiresAt,
	)

	if err == nil {
		t.Fatal("expected insufficient stock error, got nil")
	}

	t.Logf("expected reservation failure: %v", err)

	t.Cleanup(func() {
		_, err := pool.Exec(
			ctx,
			"DELETE FROM reservations WHERE id = $1",
			reservation.ID,
		)
		if err != nil {
			t.Logf("cleanup reservation failed: %v", err)
		}
	})
}
