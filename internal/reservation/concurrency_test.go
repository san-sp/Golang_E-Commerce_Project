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
	service := NewService(
		pool,
		queries,
		15*time.Minute,
	)

	// Create isolated test data.
	var productID pgtype.UUID

	err = pool.QueryRow(
		ctx,
		`INSERT INTO products (name, description)
		 VALUES ('Concurrency Test Product', 'Test product')
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
			'test-concurrency-' || gen_random_uuid()::text,
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
		VALUES ($1, 1)`,
		variantID,
	)
	if err != nil {
		t.Fatalf("create test inventory: %v", err)
	}

	// Clean up everything created by this test.
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

	// Create two concurrent requests.
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

	for result := range results {
		if result.err == nil {
			successes++
			continue
		}

		failures++

		t.Logf(
			"reservation failed as expected: %v",
			result.err,
		)
	}

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
