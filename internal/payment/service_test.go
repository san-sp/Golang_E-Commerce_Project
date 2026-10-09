package payment

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/joho/godotenv"

	"github.com/san-sp/Golang_E-Commerce_Project/internal/database"
	"github.com/san-sp/Golang_E-Commerce_Project/internal/database/db"
	"github.com/san-sp/Golang_E-Commerce_Project/internal/order"
	"github.com/san-sp/Golang_E-Commerce_Project/internal/reservation"
)

func createPaymentTestInventory(
	t *testing.T,
	pool *pgxpool.Pool,
) pgtype.UUID {
	t.Helper()

	ctx := context.Background()

	var productID pgtype.UUID

	err := pool.QueryRow(
		ctx,
		`INSERT INTO products (name, description)
		 VALUES ('Payment Test Product', 'Test product')
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
			'payment-test-' || gen_random_uuid()::text,
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

	orderService := order.NewService(pool, queries)

	variantID := createPaymentTestInventory(t, pool)

	createdReservation, err := reservationService.CreateReservation(
		ctx,
		variantID,
		1,
	)
	if err != nil {
		t.Fatalf("create test reservation: %v", err)
	}

	createdOrder, _, err := orderService.CreateOrder(
		ctx,
		[]order.ItemInput{
			{
				VariantID: variantID,
				Quantity:  1,
				UnitPrice: 899900,
			},
		},
		"INR",
	)
	if err != nil {
		t.Fatalf("create test order: %v", err)
	}

	_, err = pool.Exec(
		ctx,
		`UPDATE reservations
	 SET order_id = $1
	 WHERE id = $2`,
		createdOrder.ID,
		createdReservation.ID,
	)
	if err != nil {
		t.Fatalf("link reservation to order: %v", err)
	}

	payment, err := service.CreatePayment(
		ctx,
		createdOrder.ID,
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

		_, err = pool.Exec(
			ctx,
			"DELETE FROM orders WHERE id = $1",
			createdOrder.ID,
		)
		if err != nil {
			t.Logf("cleanup order failed: %v", err)
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

	orderService := order.NewService(pool, queries)

	variantID := createPaymentTestInventory(t, pool)

	createdReservation, err := reservationService.CreateReservation(
		ctx,
		variantID,
		1,
	)
	if err != nil {
		t.Fatalf("create test reservation: %v", err)
	}

	createdOrder, _, err := orderService.CreateOrder(
		ctx,
		[]order.ItemInput{
			{
				VariantID: variantID,
				Quantity:  1,
				UnitPrice: 899900,
			},
		},
		"INR",
	)
	if err != nil {
		t.Fatalf("create test order: %v", err)
	}

	_, err = pool.Exec(
		ctx,
		`UPDATE reservations
	 SET order_id = $1
	 WHERE id = $2`,
		createdOrder.ID,
		createdReservation.ID,
	)
	if err != nil {
		t.Fatalf("link reservation to order: %v", err)
	}

	_, err = service.CreatePayment(
		ctx,
		createdOrder.ID,
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
	 WHERE order_id = $1`,
		createdOrder.ID,
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
			`DELETE FROM payments
			WHERE order_id = $1`,
			createdOrder.ID,
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

		_, err = pool.Exec(
			ctx,
			`DELETE FROM orders
     WHERE id = $1`,
			createdOrder.ID,
		)
		if err != nil {
			t.Logf("cleanup order failed: %v", err)
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

	orderService := order.NewService(pool, queries)

	variantID := createPaymentTestInventory(t, pool)

	createdReservation, err := reservationService.CreateReservation(
		ctx,
		variantID,
		1,
	)
	if err != nil {
		t.Fatalf("create test reservation: %v", err)
	}

	createdOrder, _, err := orderService.CreateOrder(
		ctx,
		[]order.ItemInput{
			{
				VariantID: variantID,
				Quantity:  1,
				UnitPrice: 899900,
			},
		},
		"INR",
	)
	if err != nil {
		t.Fatalf("create test order: %v", err)
	}

	_, err = pool.Exec(
		ctx,
		`UPDATE reservations
	 SET order_id = $1
	 WHERE id = $2`,
		createdOrder.ID,
		createdReservation.ID,
	)
	if err != nil {
		t.Fatalf("link reservation to order: %v", err)
	}

	_, err = service.CreatePayment(
		ctx,
		createdOrder.ID,
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
	 WHERE order_id = $1`,
		createdOrder.ID,
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
		// 1. Payment
		_, err := pool.Exec(
			ctx,
			`DELETE FROM payments
			 WHERE order_id = $1`,
			createdOrder.ID,
		)
		if err != nil {
			t.Logf("cleanup payment failed: %v", err)
		}

		// 2. Reservation
		_, err = pool.Exec(
			ctx,
			"DELETE FROM reservations WHERE id = $1",
			createdReservation.ID,
		)
		if err != nil {
			t.Logf("cleanup reservation failed: %v", err)
		}

		// 3. Order
		_, err = pool.Exec(
			ctx,
			`DELETE FROM orders
		 WHERE id = $1`,
			createdOrder.ID,
		)
		if err != nil {
			t.Logf("cleanup order failed: %v", err)
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

	orderService := order.NewService(pool, queries)

	variantID := createPaymentTestInventory(t, pool)

	createdReservation, err := reservationService.CreateReservation(
		ctx,
		variantID,
		1,
	)
	if err != nil {
		t.Fatalf("create test reservation: %v", err)
	}

	createdOrder, _, err := orderService.CreateOrder(
		ctx,
		[]order.ItemInput{
			{
				VariantID: variantID,
				Quantity:  1,
				UnitPrice: 899900,
			},
		},
		"INR",
	)
	if err != nil {
		t.Fatalf("create test order: %v", err)
	}

	_, err = pool.Exec(
		ctx,
		`UPDATE reservations
	 SET order_id = $1
	 WHERE id = $2`,
		createdOrder.ID,
		createdReservation.ID,
	)
	if err != nil {
		t.Fatalf("link reservation to order: %v", err)
	}

	payment, err := service.CreatePayment(
		ctx,
		createdOrder.ID,
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

		_, err = pool.Exec(
			ctx,
			"DELETE FROM orders WHERE id = $1",
			createdOrder.ID,
		)
		if err != nil {
			t.Logf("cleanup order failed: %v", err)
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
