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

func TestWorkerProcessOnce(t *testing.T) {
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

	reservation, err := queries.CreateReservation(ctx, db.CreateReservationParams{
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

	worker := NewWorker(
		queries,
		5*time.Second,
		100,
	)

	err = worker.ProcessOnce(ctx)
	if err != nil {
		t.Fatalf("process reservation expiration: %v", err)
	}

	updatedReservation, err := queries.GetReservation(ctx, reservation.ID)
	if err != nil {
		t.Fatalf("get reservation after expiration: %v", err)
	}

	if updatedReservation.Status != "EXPIRED" {
		t.Fatalf(
			"expected reservation status EXPIRED, got %s",
			updatedReservation.Status,
		)
	}
}
