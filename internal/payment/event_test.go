package payment

import (
	"context"
	"errors"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/joho/godotenv"

	"github.com/san-sp/Golang_E-Commerce_Project/internal/database"
	"github.com/san-sp/Golang_E-Commerce_Project/internal/database/db"
	"github.com/san-sp/Golang_E-Commerce_Project/internal/order"
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
	// Create a PENDING payment.
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

func TestProcessPaymentWebhook(t *testing.T) {
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
	// Create isolated test product.
	var productID pgtype.UUID
	err = pool.QueryRow(
		ctx,
		`INSERT INTO products (name, description)
     VALUES ('Webhook Test Product', 'Test product')
     RETURNING id`,
	).Scan(&productID)
	if err != nil {
		t.Fatalf("create test product: %v", err)
	}
	// Create isolated test variant.
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
      'webhook-test-' || gen_random_uuid()::text,
      899900
    )
    RETURNING id`,
		productID,
	).Scan(&variantID)
	if err != nil {
		t.Fatalf("create test variant: %v", err)
	}
	// Create inventory with one item.
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
	var paymentID pgtype.UUID
	var paymentIDString string
	// Clean up everything created by this test.
	t.Cleanup(func() {
		_, err := pool.Exec(
			ctx,
			`DELETE FROM outbox_events
       WHERE event_type = 'PAYMENT_SUCCEEDED'
         AND payload->>'payment_id' = $1`,
			paymentIDString,
		)
		if err != nil {
			t.Logf("cleanup outbox events failed: %v", err)
		}
		_, err = pool.Exec(
			ctx,
			`DELETE FROM payment_events
       WHERE payment_id = $1`,
			paymentID,
		)
		if err != nil {
			t.Logf("cleanup payment events failed: %v", err)
		}
		_, err = pool.Exec(
			ctx,
			`DELETE FROM payments
       WHERE id = $1`,
			paymentID,
		)
		if err != nil {
			t.Logf("cleanup payment failed: %v", err)
		}
		_, err = pool.Exec(
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
			t.Logf("cleanup variant failed: %v", err)
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
	// Create a test order.
	orderService := order.NewService(pool, queries)
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

	// Create an ACTIVE reservation.
	createdReservation, err := reservationService.CreateReservation(
		ctx,
		variantID,
		1,
	)
	if err != nil {
		t.Fatalf("create test reservation: %v", err)
	}
	// Link the reservation to the order.
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
	// Create a PENDING payment.
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
	paymentID = payment.ID
	paymentIDString = paymentID.String()

	t.Cleanup(func() {
		_, err := pool.Exec(
			ctx,
			`DELETE FROM outbox_events
         WHERE event_type = 'PAYMENT_SUCCEEDED'
           AND payload->>'payment_id' = $1`,
			paymentID.String(),
		)
		if err != nil {
			t.Logf("cleanup outbox events failed: %v", err)
		}

		_, err = pool.Exec(
			ctx,
			`DELETE FROM payment_events
         WHERE payment_id = $1`,
			paymentID,
		)
		if err != nil {
			t.Logf("cleanup payment events failed: %v", err)
		}

		_, err = pool.Exec(
			ctx,
			`DELETE FROM payments
         WHERE id = $1`,
			paymentID,
		)
		if err != nil {
			t.Logf("cleanup payment failed: %v", err)
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

	if payment.Status != "PENDING" {
		t.Fatalf(
			"expected payment status PENDING, got %s",
			payment.Status,
		)
	}
	eventID := fmt.Sprintf(
		"webhook-test-%d",
		time.Now().UnixNano(),
	)
	webhook := PaymentWebhook{
		EventID:   eventID,
		EventType: "payment.succeeded",
		PaymentID: payment.ID,
	}

	// Process the webhook.
	updatedPayment, err := service.ProcessPaymentWebhook(
		ctx,
		webhook,
	)
	if err != nil {
		t.Fatalf("process payment webhook: %v", err)
	}
	// Payment should now be SUCCEEDED.
	if updatedPayment.Status != "SUCCEEDED" {
		t.Fatalf(
			"expected payment status SUCCEEDED, got %s",
			updatedPayment.Status,
		)
	}

	// Reservation should now be CONFIRMED.
	var reservationStatus string
	err = pool.QueryRow(
		ctx,
		`SELECT status
     FROM reservations
     WHERE id = $1`,
		createdReservation.ID,
	).Scan(&reservationStatus)
	if err != nil {
		t.Fatalf("query reservation status: %v", err)
	}
	if reservationStatus != "CONFIRMED" {
		t.Fatalf(
			"expected reservation status CONFIRMED, got %s",
			reservationStatus,
		)
	}

	// Inventory should now be consumed.
	var inventoryQuantity int64

	err = pool.QueryRow(
		ctx,
		`SELECT quantity
 FROM inventory
 WHERE variant_id = $1`,
		variantID,
	).Scan(&inventoryQuantity)
	if err != nil {
		t.Fatalf("query inventory quantity: %v", err)
	}

	if inventoryQuantity != 0 {
		t.Fatalf(
			"expected inventory quantity 0, got %d",
			inventoryQuantity,
		)
	}

	// Order should now be CONFIRMED.
	var orderStatus string

	err = pool.QueryRow(
		ctx,
		`SELECT status
     FROM orders
     WHERE id = $1`,
		createdOrder.ID,
	).Scan(&orderStatus)
	if err != nil {
		t.Fatalf("query order status: %v", err)
	}

	if orderStatus != "CONFIRMED" {
		t.Fatalf(
			"expected order status CONFIRMED, got %s",
			orderStatus,
		)
	}

	// Payment event should have been recorded.
	var paymentEventCount int
	err = pool.QueryRow(
		ctx,
		`SELECT COUNT(*)
     FROM payment_events
     WHERE event_id = $1`,
		eventID,
	).Scan(&paymentEventCount)
	if err != nil {
		t.Fatalf("query payment event: %v", err)
	}
	if paymentEventCount != 1 {
		t.Fatalf(
			"expected 1 payment event, got %d",
			paymentEventCount,
		)
	}
	// Outbox event should have been created.
	var outboxCount int
	err = pool.QueryRow(
		ctx,
		`SELECT COUNT(*)
     FROM outbox_events
     WHERE event_type = 'PAYMENT_SUCCEEDED'
       AND payload->>'payment_id' = $1`,
		paymentIDString,
	).Scan(&outboxCount)
	if err != nil {
		t.Fatalf("query outbox event: %v", err)
	}
	if outboxCount != 1 {
		t.Fatalf(
			"expected 1 PAYMENT_SUCCEEDED outbox event, got %d",
			outboxCount,
		)
	}
	// Send the exact same webhook again.
	_, err = service.ProcessPaymentWebhook(
		ctx,
		webhook,
	)
	if !errors.Is(err, ErrDuplicateWebhook) {
		t.Fatalf(
			"expected ErrDuplicateWebhook, got %v",
			err,
		)
	}
	// The duplicate webhook must not create another outbox event.
	err = pool.QueryRow(
		ctx,
		`SELECT COUNT(*)
     FROM outbox_events
     WHERE event_type = 'PAYMENT_SUCCEEDED'
       AND payload->>'payment_id' = $1`,
		paymentIDString,
	).Scan(&outboxCount)
	if err != nil {
		t.Fatalf("query outbox event after duplicate: %v", err)
	}
	if outboxCount != 1 {
		t.Fatalf(
			"expected exactly 1 outbox event after duplicate webhook, got %d",
			outboxCount,
		)
	}
}

func TestProcessPaymentWebhookRollback(t *testing.T) {
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
	// Create isolated test product.
	var productID pgtype.UUID
	err = pool.QueryRow(
		ctx,
		`INSERT INTO products (name, description)
     VALUES ('Webhook Rollback Test Product', 'Test product')
     RETURNING id`,
	).Scan(&productID)
	if err != nil {
		t.Fatalf("create test product: %v", err)
	}
	// Create isolated test variant.
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
      'webhook-rollback-' || gen_random_uuid()::text,
      899900
    )
    RETURNING id`,
		productID,
	).Scan(&variantID)
	if err != nil {
		t.Fatalf("create test variant: %v", err)
	}
	// Create inventory.
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
	var paymentID pgtype.UUID
	// Clean up everything created by this test.
	t.Cleanup(func() {
		_, err := pool.Exec(
			ctx,
			`DELETE FROM outbox_events
       WHERE event_type = 'PAYMENT_SUCCEEDED'
         AND payload->>'payment_id' = $1`,
			paymentID.String(),
		)
		if err != nil {
			t.Logf("cleanup outbox events failed: %v", err)
		}
		_, err = pool.Exec(
			ctx,
			`DELETE FROM payment_events
       WHERE payment_id = $1`,
			paymentID,
		)
		if err != nil {
			t.Logf("cleanup payment events failed: %v", err)
		}
		_, err = pool.Exec(
			ctx,
			`DELETE FROM payments
       WHERE id = $1`,
			paymentID,
		)
		if err != nil {
			t.Logf("cleanup payment failed: %v", err)
		}
		_, err = pool.Exec(
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
			t.Logf("cleanup variant failed: %v", err)
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
	// Create a reservation that will be expired.
	createdReservation, err := reservationService.CreateReservation(
		ctx,
		variantID,
		1,
	)
	if err != nil {
		t.Fatalf("create test reservation: %v", err)
	}
	_, err = pool.Exec(
		ctx,
		`UPDATE reservations
   SET expires_at = NOW() - INTERVAL '10 minutes'
   WHERE id = $1`,
		createdReservation.ID,
	)
	if err != nil {
		t.Fatalf("expire test reservation: %v", err)
	}
	// Create a PENDING payment.
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
	// Create a PENDING payment.
	createdPayment, err := service.CreatePayment(
		ctx,
		createdOrder.ID,
		"mock",
		899900,
		"INR",
	)
	if err != nil {
		t.Fatalf("create payment: %v", err)
	}
	paymentID = createdPayment.ID

	t.Cleanup(func() {
		_, err := pool.Exec(
			ctx,
			`DELETE FROM payment_events
         WHERE payment_id = $1`,
			paymentID,
		)
		if err != nil {
			t.Logf("cleanup payment events failed: %v", err)
		}

		_, err = pool.Exec(
			ctx,
			`DELETE FROM payments
         WHERE id = $1`,
			paymentID,
		)
		if err != nil {
			t.Logf("cleanup payment failed: %v", err)
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

	eventID := fmt.Sprintf(
		"rollback-test-%d",
		time.Now().UnixNano(),
	)
	webhook := PaymentWebhook{
		EventID:   eventID,
		EventType: "payment.succeeded",
		PaymentID: createdPayment.ID,
	}
	// The webhook must fail because the reservation is expired.
	_, err = service.ProcessPaymentWebhook(ctx, webhook)
	if err == nil {
		t.Fatal("expected webhook processing to fail")
	}
	if !errors.Is(err, ErrReservationExpired) {
		t.Fatalf(
			"expected ErrReservationExpired, got %v",
			err,
		)
	}
	t.Logf("webhook failed as expected: %v", err)
	// Payment must remain PENDING because the transaction rolled back.
	var paymentStatus string
	err = pool.QueryRow(
		ctx,
		`SELECT status
     FROM payments
     WHERE id = $1`,
		createdPayment.ID,
	).Scan(&paymentStatus)
	if err != nil {
		t.Fatalf("query payment status: %v", err)
	}
	if paymentStatus != "PENDING" {
		t.Fatalf(
			"expected payment status PENDING after rollback, got %s",
			paymentStatus,
		)
	}
	// Payment event must not exist because its insert was rolled back.
	var paymentEventCount int
	err = pool.QueryRow(
		ctx,
		`SELECT COUNT(*)
     FROM payment_events
     WHERE event_id = $1`,
		eventID,
	).Scan(&paymentEventCount)
	if err != nil {
		t.Fatalf("query payment event: %v", err)
	}
	if paymentEventCount != 0 {
		t.Fatalf(
			"expected 0 payment events after rollback, got %d",
			paymentEventCount,
		)
	}
	// Outbox event must not exist because its insert was rolled back.
	var outboxCount int
	err = pool.QueryRow(
		ctx,
		`SELECT COUNT(*)
     FROM outbox_events
     WHERE event_type = 'PAYMENT_SUCCEEDED'
         AND payload->>'payment_id' = $1`,
		createdPayment.ID.String(),
	).Scan(&outboxCount)
	if err != nil {
		t.Fatalf("query outbox event: %v", err)
	}
	if outboxCount != 0 {
		t.Fatalf(
			"expected 0 outbox events after rollback, got %d",
			outboxCount,
		)
	}
}

func TestProcessPaymentWebhookInventoryRollback(t *testing.T) {
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

	// Create isolated test product.
	var productID pgtype.UUID

	err = pool.QueryRow(
		ctx,
		`INSERT INTO products (name, description)
		 VALUES ('Inventory Rollback Test Product', 'Test product')
		 RETURNING id`,
	).Scan(&productID)
	if err != nil {
		t.Fatalf("create test product: %v", err)
	}

	// Create isolated test variant.
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
			'inventory-rollback-' || gen_random_uuid()::text,
			899900
		)
		RETURNING id`,
		productID,
	).Scan(&variantID)
	if err != nil {
		t.Fatalf("create test variant: %v", err)
	}

	// Create inventory with one item.
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

	var paymentID pgtype.UUID

	// Clean up everything created by this test.
	t.Cleanup(func() {
		_, err := pool.Exec(
			ctx,
			`DELETE FROM outbox_events
			 WHERE event_type = 'PAYMENT_SUCCEEDED'
			   AND payload->>'payment_id' = $1`,
			paymentID.String(),
		)
		if err != nil {
			t.Logf("cleanup outbox events failed: %v", err)
		}

		_, err = pool.Exec(
			ctx,
			`DELETE FROM payment_events
			 WHERE payment_id = $1`,
			paymentID,
		)
		if err != nil {
			t.Logf("cleanup payment events failed: %v", err)
		}

		_, err = pool.Exec(
			ctx,
			`DELETE FROM payments
			 WHERE id = $1`,
			paymentID,
		)
		if err != nil {
			t.Logf("cleanup payment failed: %v", err)
		}

		_, err = pool.Exec(
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
			t.Logf("cleanup variant failed: %v", err)
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

	// Create an order.
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

	// Create an ACTIVE reservation.
	createdReservation, err := reservationService.CreateReservation(
		ctx,
		variantID,
		1,
	)
	if err != nil {
		t.Fatalf("create test reservation: %v", err)
	}

	// Link the reservation to the order.
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

	// Confirm the order before processing the webhook.
	//
	// This intentionally makes ConfirmOrder fail later in the
	// webhook transaction, after inventory has already been consumed.
	_, err = queries.ConfirmOrder(
		ctx,
		createdOrder.ID,
	)
	if err != nil {
		t.Fatalf("confirm test order: %v", err)
	}

	// Create a PENDING payment.
	createdPayment, err := service.CreatePayment(
		ctx,
		createdOrder.ID,
		"mock",
		899900,
		"INR",
	)
	if err != nil {
		t.Fatalf("create payment: %v", err)
	}

	paymentID = createdPayment.ID

	t.Cleanup(func() {
		_, err := pool.Exec(
			ctx,
			`DELETE FROM payment_events
         WHERE payment_id = $1`,
			paymentID,
		)
		if err != nil {
			t.Logf("cleanup payment events failed: %v", err)
		}

		_, err = pool.Exec(
			ctx,
			`DELETE FROM payments
         WHERE id = $1`,
			paymentID,
		)
		if err != nil {
			t.Logf("cleanup payment failed: %v", err)
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

	eventID := fmt.Sprintf(
		"inventory-rollback-test-%d",
		time.Now().UnixNano(),
	)

	webhook := PaymentWebhook{
		EventID:   eventID,
		EventType: "payment.succeeded",
		PaymentID: createdPayment.ID,
	}

	// The webhook must fail when ConfirmOrder is attempted.
	_, err = service.ProcessPaymentWebhook(
		ctx,
		webhook,
	)
	if err == nil {
		t.Fatal("expected webhook processing to fail")
	}

	if !errors.Is(err, ErrInvalidOrderState) {
		t.Fatalf(
			"expected ErrInvalidOrderState, got %v",
			err,
		)
	}

	t.Logf(
		"webhook failed as expected: %v",
		err,
	)

	// Inventory must remain unchanged because the transaction rolled back.
	var inventoryQuantity int64

	err = pool.QueryRow(
		ctx,
		`SELECT quantity
		 FROM inventory
		 WHERE variant_id = $1`,
		variantID,
	).Scan(&inventoryQuantity)
	if err != nil {
		t.Fatalf("query inventory quantity: %v", err)
	}

	if inventoryQuantity != 1 {
		t.Fatalf(
			"expected inventory quantity 1 after rollback, got %d",
			inventoryQuantity,
		)
	}

	// Reservation must remain ACTIVE because the transaction rolled back.
	var reservationStatus string

	err = pool.QueryRow(
		ctx,
		`SELECT status
		 FROM reservations
		 WHERE id = $1`,
		createdReservation.ID,
	).Scan(&reservationStatus)
	if err != nil {
		t.Fatalf("query reservation status: %v", err)
	}

	if reservationStatus != "ACTIVE" {
		t.Fatalf(
			"expected reservation status ACTIVE after rollback, got %s",
			reservationStatus,
		)
	}

	// Payment must remain PENDING because the transaction rolled back.
	var paymentStatus string

	err = pool.QueryRow(
		ctx,
		`SELECT status
		 FROM payments
		 WHERE id = $1`,
		createdPayment.ID,
	).Scan(&paymentStatus)
	if err != nil {
		t.Fatalf("query payment status: %v", err)
	}

	if paymentStatus != "PENDING" {
		t.Fatalf(
			"expected payment status PENDING after rollback, got %s",
			paymentStatus,
		)
	}

	// Payment event must not exist because its insert was rolled back.
	var paymentEventCount int

	err = pool.QueryRow(
		ctx,
		`SELECT COUNT(*)
		 FROM payment_events
		 WHERE event_id = $1`,
		eventID,
	).Scan(&paymentEventCount)
	if err != nil {
		t.Fatalf("query payment event: %v", err)
	}

	if paymentEventCount != 0 {
		t.Fatalf(
			"expected 0 payment events after rollback, got %d",
			paymentEventCount,
		)
	}

	// Outbox event must not exist because its insert was rolled back.
	var outboxCount int

	err = pool.QueryRow(
		ctx,
		`SELECT COUNT(*)
		 FROM outbox_events
		 WHERE event_type = 'PAYMENT_SUCCEEDED'
		   AND payload->>'payment_id' = $1`,
		createdPayment.ID.String(),
	).Scan(&outboxCount)
	if err != nil {
		t.Fatalf("query outbox event: %v", err)
	}

	if outboxCount != 0 {
		t.Fatalf(
			"expected 0 outbox events after rollback, got %d",
			outboxCount,
		)
	}
}
