package reservation

import (
	"context"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/joho/godotenv"

	"github.com/san-sp/Golang_E-Commerce_Project/internal/database"
	"github.com/san-sp/Golang_E-Commerce_Project/internal/database/db"
)

type reservationResult struct {
	reservation db.Reservation
	err         error
}

func TestConcurrentReservations(t *testing.T) {
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

	var originalQuantity int64

	err = pool.QueryRow(
		ctx,
		`SELECT quantity
		 FROM inventory
		 WHERE variant_id = $1`,
		variantID,
	).Scan(&originalQuantity)
	if err != nil {
		t.Fatalf("get original inventory quantity: %v", err)
	}

	var activeReservations int64

	err = pool.QueryRow(
		ctx,
		`SELECT COUNT(*)
		 FROM reservations
		 WHERE variant_id = $1
		   AND status = 'ACTIVE'
		   AND expires_at > NOW()`,
		variantID,
	).Scan(&activeReservations)
	if err != nil {
		t.Fatalf("count active reservations: %v", err)
	}

	if activeReservations != 0 {
		t.Fatalf(
			"expected no active reservations before concurrency test, found %d",
			activeReservations,
		)
	}

	_, err = pool.Exec(
		ctx,
		`UPDATE inventory
		 SET quantity = 1,
		     updated_at = NOW()
		 WHERE variant_id = $1`,
		variantID,
	)
	if err != nil {
		t.Fatalf("set test inventory quantity: %v", err)
	}

	// Always restore the original inventory quantity.
	t.Cleanup(func() {
		_, err := pool.Exec(
			ctx,
			`UPDATE inventory
			 SET quantity = $1,
			     updated_at = NOW()
			 WHERE variant_id = $2`,
			originalQuantity,
			variantID,
		)
		if err != nil {
			t.Logf("restore inventory failed: %v", err)
		}
	})

	start := make(chan struct{})
	results := make(chan reservationResult, 2)

	var wg sync.WaitGroup
	wg.Add(2)

	for i := 0; i < 2; i++ {
		go func() {
			defer wg.Done()

			<-start

			reservation, err := service.CreateReservation(
				ctx,
				variantID,
				1,
				expiresAt,
			)

			results <- reservationResult{
				reservation: reservation,
				err:         err,
			}
		}()
	}

	close(start)

	wg.Wait()
	close(results)

	successes := 0
	failures := 0

	var createdReservationIDs []pgtype.UUID

	for result := range results {
		if result.err == nil {
			successes++

			createdReservationIDs = append(
				createdReservationIDs,
				result.reservation.ID,
			)

			continue
		}

		failures++

		t.Logf(
			"reservation failed as expected: %v",
			result.err,
		)
	}

	// Clean up only the reservations created by this test.
	t.Cleanup(func() {
		for _, reservationID := range createdReservationIDs {
			_, err := pool.Exec(
				ctx,
				`DELETE FROM reservations
				 WHERE id = $1`,
				reservationID,
			)

			if err != nil {
				t.Logf(
					"cleanup reservation %s failed: %v",
					reservationID,
					err,
				)
			}
		}
	})

	if successes != 1 {
		t.Fatalf(
			"expected exactly 1 successful reservation, got %d",
			successes,
		)
	}

	if failures != 1 {
		t.Fatalf(
			"expected exactly 1 failed reservation, got %d",
			failures,
		)
	}
}
