package payment

import (
	"context"
	"errors"
	"fmt"
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

type refundTestFixture struct {
	pool        *pgxpool.Pool
	queries     *db.Queries
	provider    *MockProvider
	refund      *RefundService
	payment     db.Payment
	orderID     pgtype.UUID
	variantID   pgtype.UUID
	reservation pgtype.UUID
}

func setupRefundTest(t *testing.T) *refundTestFixture {
	t.Helper()

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

	queries := db.New(pool)
	provider := NewMockProvider()

	paymentService := NewService(
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
		 VALUES ('Refund Test Product', 'Test product')
		 RETURNING id`,
	).Scan(&productID)
	if err != nil {
		pool.Close()
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
			'refund-test-' || gen_random_uuid()::text,
			899900
		)
		RETURNING id`,
		productID,
	).Scan(&variantID)
	if err != nil {
		pool.Close()
		t.Fatalf("create test variant: %v", err)
	}

	// Create one unit of inventory.
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
		pool.Close()
		t.Fatalf("create test inventory: %v", err)
	}

	// Create order.
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
		pool.Close()
		t.Fatalf("create test order: %v", err)
	}

	// Create reservation.
	createdReservation, err := reservationService.CreateReservation(
		ctx,
		variantID,
		1,
	)
	if err != nil {
		pool.Close()
		t.Fatalf("create test reservation: %v", err)
	}

	// Link reservation to order.
	_, err = pool.Exec(
		ctx,
		`UPDATE reservations
		 SET order_id = $1
		 WHERE id = $2`,
		createdOrder.ID,
		createdReservation.ID,
	)
	if err != nil {
		pool.Close()
		t.Fatalf("link reservation to order: %v", err)
	}

	// Create payment.
	payment, err := paymentService.CreatePayment(
		ctx,
		createdOrder.ID,
		"mock",
		899900,
		"INR",
	)
	if err != nil {
		pool.Close()
		t.Fatalf("create payment: %v", err)
	}

	// Process the real payment webhook.
	eventID := fmt.Sprintf(
		"refund-test-webhook-%d",
		time.Now().UnixNano(),
	)

	_, err = paymentService.ProcessPaymentWebhook(
		ctx,
		PaymentWebhook{
			EventID:   eventID,
			EventType: "payment.succeeded",
			PaymentID: payment.ID,
		},
	)
	if err != nil {
		pool.Close()
		t.Fatalf("process payment webhook: %v", err)
	}

	refundService := NewRefundService(
		pool,
		queries,
		provider,
	)

	fixture := &refundTestFixture{
		pool:        pool,
		queries:     queries,
		provider:    provider,
		refund:      refundService,
		payment:     payment,
		orderID:     createdOrder.ID,
		variantID:   variantID,
		reservation: createdReservation.ID,
	}

	t.Cleanup(func() {
		cleanupRefundFixture(t, fixture, productID, eventID)
	})

	return fixture
}

func cleanupRefundFixture(
	t *testing.T,
	fixture *refundTestFixture,
	productID pgtype.UUID,
	eventID string,
) {
	t.Helper()

	ctx := context.Background()

	// Refunds first because they reference payments.
	// Outbox events first because refund IDs are stored in the payload.
	_, err := fixture.pool.Exec(
		ctx,
		`DELETE FROM outbox_events
     WHERE payload->>'payment_id' = $1
        OR payload->>'refund_id' IN (
            SELECT id::text
            FROM refunds
            WHERE payment_id = $2
        )`,
		fixture.payment.ID.String(),
		fixture.payment.ID,
	)
	if err != nil {
		t.Logf("cleanup outbox events failed: %v", err)
	}

	// Refunds reference payments.
	_, err = fixture.pool.Exec(
		ctx,
		`DELETE FROM refunds
     WHERE payment_id = $1`,
		fixture.payment.ID,
	)
	if err != nil {
		t.Logf("cleanup refunds failed: %v", err)
	}

	_, err = fixture.pool.Exec(
		ctx,
		`DELETE FROM payment_events
		 WHERE payment_id = $1`,
		fixture.payment.ID,
	)
	if err != nil {
		t.Logf("cleanup payment events failed: %v", err)
	}

	_, err = fixture.pool.Exec(
		ctx,
		`DELETE FROM payments
		 WHERE id = $1`,
		fixture.payment.ID,
	)
	if err != nil {
		t.Logf("cleanup payment failed: %v", err)
	}

	_, err = fixture.pool.Exec(
		ctx,
		`DELETE FROM reservations
		 WHERE id = $1`,
		fixture.reservation,
	)
	if err != nil {
		t.Logf("cleanup reservation failed: %v", err)
	}

	_, err = fixture.pool.Exec(
		ctx,
		`DELETE FROM inventory
		 WHERE variant_id = $1`,
		fixture.variantID,
	)
	if err != nil {
		t.Logf("cleanup inventory failed: %v", err)
	}

	_, err = fixture.pool.Exec(
		ctx,
		`DELETE FROM orders
		 WHERE id = $1`,
		fixture.orderID,
	)
	if err != nil {
		t.Logf("cleanup order failed: %v", err)
	}

	_, err = fixture.pool.Exec(
		ctx,
		`DELETE FROM product_variants
		 WHERE id = $1`,
		fixture.variantID,
	)
	if err != nil {
		t.Logf("cleanup variant failed: %v", err)
	}

	_, err = fixture.pool.Exec(
		ctx,
		`DELETE FROM products
		 WHERE id = $1`,
		productID,
	)
	if err != nil {
		t.Logf("cleanup product failed: %v", err)
	}

	fixture.pool.Close()

	_ = eventID
}

func TestCreateRefund(t *testing.T) {
	fixture := setupRefundTest(t)
	ctx := context.Background()

	// Before refund:
	// payment = SUCCEEDED
	// order = CONFIRMED
	// reservation = CONFIRMED
	// inventory = 0

	refund, err := fixture.refund.CreateRefund(
		ctx,
		fixture.payment.ID,
		"refund-test-success",
	)
	if err != nil {
		t.Fatalf("create refund: %v", err)
	}

	if refund.Status != "SUCCEEDED" {
		t.Fatalf(
			"expected refund status SUCCEEDED, got %s",
			refund.Status,
		)
	}

	if refund.Amount != 899900 {
		t.Fatalf(
			"expected refund amount 899900, got %d",
			refund.Amount,
		)
	}

	if refund.Currency != "INR" {
		t.Fatalf(
			"expected refund currency INR, got %s",
			refund.Currency,
		)
	}

	if !refund.ProviderRefundID.Valid {
		t.Fatal("expected provider refund ID")
	}

	// Inventory should be restored.
	var inventoryQuantity int64

	err = fixture.pool.QueryRow(
		ctx,
		`SELECT quantity
		 FROM inventory
		 WHERE variant_id = $1`,
		fixture.variantID,
	).Scan(&inventoryQuantity)
	if err != nil {
		t.Fatalf("query inventory quantity: %v", err)
	}

	if inventoryQuantity != 1 {
		t.Fatalf(
			"expected inventory quantity 1, got %d",
			inventoryQuantity,
		)
	}

	// Order should become REFUNDED.
	var orderStatus string

	err = fixture.pool.QueryRow(
		ctx,
		`SELECT status
		 FROM orders
		 WHERE id = $1`,
		fixture.orderID,
	).Scan(&orderStatus)
	if err != nil {
		t.Fatalf("query order status: %v", err)
	}

	if orderStatus != "REFUNDED" {
		t.Fatalf(
			"expected order status REFUNDED, got %s",
			orderStatus,
		)
	}

	// Reservation remains CONFIRMED as historical evidence.
	var reservationStatus string

	err = fixture.pool.QueryRow(
		ctx,
		`SELECT status
		 FROM reservations
		 WHERE id = $1`,
		fixture.reservation,
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

	// Payment remains SUCCEEDED.
	var paymentStatus string

	err = fixture.pool.QueryRow(
		ctx,
		`SELECT status
		 FROM payments
		 WHERE id = $1`,
		fixture.payment.ID,
	).Scan(&paymentStatus)
	if err != nil {
		t.Fatalf("query payment status: %v", err)
	}

	if paymentStatus != "SUCCEEDED" {
		t.Fatalf(
			"expected payment status SUCCEEDED, got %s",
			paymentStatus,
		)
	}

	// Exactly one refund exists.
	var refundCount int

	err = fixture.pool.QueryRow(
		ctx,
		`SELECT COUNT(*)
		 FROM refunds
		 WHERE payment_id = $1`,
		fixture.payment.ID,
	).Scan(&refundCount)
	if err != nil {
		t.Fatalf("query refund count: %v", err)
	}

	if refundCount != 1 {
		t.Fatalf(
			"expected 1 refund, got %d",
			refundCount,
		)
	}

	// PAYMENT_REFUNDED outbox event should exist.
	var outboxCount int

	err = fixture.pool.QueryRow(
		ctx,
		`SELECT COUNT(*)
		 FROM outbox_events
		 WHERE event_type = 'PAYMENT_REFUNDED'
		   AND payload->>'refund_id' = $1`,
		refund.ID.String(),
	).Scan(&outboxCount)
	if err != nil {
		t.Fatalf("query refund outbox count: %v", err)
	}

	if outboxCount != 1 {
		t.Fatalf(
			"expected 1 PAYMENT_REFUNDED outbox event, got %d",
			outboxCount,
		)
	}
}

func TestCreateRefundIdempotency(t *testing.T) {
	fixture := setupRefundTest(t)
	ctx := context.Background()

	const idempotencyKey = "refund-test-idempotency"

	firstRefund, err := fixture.refund.CreateRefund(
		ctx,
		fixture.payment.ID,
		idempotencyKey,
	)
	if err != nil {
		t.Fatalf("create first refund: %v", err)
	}

	secondRefund, err := fixture.refund.CreateRefund(
		ctx,
		fixture.payment.ID,
		idempotencyKey,
	)
	if err != nil {
		t.Fatalf("create second refund: %v", err)
	}

	if firstRefund.ID != secondRefund.ID {
		t.Fatalf(
			"expected same refund ID, got %s and %s",
			firstRefund.ID,
			secondRefund.ID,
		)
	}

	var refundCount int

	err = fixture.pool.QueryRow(
		ctx,
		`SELECT COUNT(*)
		 FROM refunds
		 WHERE payment_id = $1`,
		fixture.payment.ID,
	).Scan(&refundCount)
	if err != nil {
		t.Fatalf("query refund count: %v", err)
	}

	if refundCount != 1 {
		t.Fatalf(
			"expected exactly 1 refund, got %d",
			refundCount,
		)
	}

	var inventoryQuantity int64

	err = fixture.pool.QueryRow(
		ctx,
		`SELECT quantity
		 FROM inventory
		 WHERE variant_id = $1`,
		fixture.variantID,
	).Scan(&inventoryQuantity)
	if err != nil {
		t.Fatalf("query inventory quantity: %v", err)
	}

	if inventoryQuantity != 1 {
		t.Fatalf(
			"expected inventory quantity to remain 1, got %d",
			inventoryQuantity,
		)
	}
}

func TestCreateRefundDifferentIdempotencyKeyAfterRefund(t *testing.T) {
	fixture := setupRefundTest(t)
	ctx := context.Background()

	_, err := fixture.refund.CreateRefund(
		ctx,
		fixture.payment.ID,
		"refund-key-first",
	)
	if err != nil {
		t.Fatalf("create first refund: %v", err)
	}

	_, err = fixture.refund.CreateRefund(
		ctx,
		fixture.payment.ID,
		"refund-key-second",
	)
	if !errors.Is(err, ErrOrderNotRefundable) {
		t.Fatalf(
			"expected ErrOrderNotRefundable, got %v",
			err,
		)
	}
}

func TestCreateRefundProviderFailure(t *testing.T) {
	fixture := setupRefundTest(t)
	ctx := context.Background()

	fixture.provider.SetFailure(true)

	refund, err := fixture.refund.CreateRefund(
		ctx,
		fixture.payment.ID,
		"refund-test-failure",
	)
	if !errors.Is(err, ErrProviderFailed) {
		t.Fatalf(
			"expected ErrProviderFailed, got %v",
			err,
		)
	}

	if refund.Status != "FAILED" {
		t.Fatalf(
			"expected refund status FAILED, got %s",
			refund.Status,
		)
	}

	var inventoryQuantity int64

	err = fixture.pool.QueryRow(
		ctx,
		`SELECT quantity
		 FROM inventory
		 WHERE variant_id = $1`,
		fixture.variantID,
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

	var orderStatus string

	err = fixture.pool.QueryRow(
		ctx,
		`SELECT status
		 FROM orders
		 WHERE id = $1`,
		fixture.orderID,
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
}

func TestCreateRefundProviderUnknown(t *testing.T) {
	fixture := setupRefundTest(t)
	ctx := context.Background()

	fixture.provider.SetUnknown()

	refund, err := fixture.refund.CreateRefund(
		ctx,
		fixture.payment.ID,
		"refund-test-unknown",
	)
	if !errors.Is(err, ErrProviderUnknown) {
		t.Fatalf(
			"expected ErrProviderUnknown, got %v",
			err,
		)
	}

	if refund.Status != "PENDING" {
		t.Fatalf(
			"expected refund status PENDING, got %s",
			refund.Status,
		)
	}

	var inventoryQuantity int64

	err = fixture.pool.QueryRow(
		ctx,
		`SELECT quantity
		 FROM inventory
		 WHERE variant_id = $1`,
		fixture.variantID,
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

	var orderStatus string

	err = fixture.pool.QueryRow(
		ctx,
		`SELECT status
		 FROM orders
		 WHERE id = $1`,
		fixture.orderID,
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
}

func TestCreateRefundRollback(t *testing.T) {
	fixture := setupRefundTest(t)
	ctx := context.Background()

	// Remove the inventory row after payment succeeded.
	// The provider refund will succeed, but the local finalization
	// transaction must fail while restoring inventory.
	_, err := fixture.pool.Exec(
		ctx,
		`DELETE FROM inventory
		 WHERE variant_id = $1`,
		fixture.variantID,
	)
	if err != nil {
		t.Fatalf("delete inventory for rollback test: %v", err)
	}

	_, err = fixture.refund.CreateRefund(
		ctx,
		fixture.payment.ID,
		"refund-test-rollback",
	)
	if err == nil {
		t.Fatal("expected refund finalization error")
	}

	// The refund must have rolled back to PENDING.
	var refundStatus string

	err = fixture.pool.QueryRow(
		ctx,
		`SELECT status
		 FROM refunds
		 WHERE payment_id = $1`,
		fixture.payment.ID,
	).Scan(&refundStatus)
	if err != nil {
		t.Fatalf("query refund status: %v", err)
	}

	if refundStatus != "PENDING" {
		t.Fatalf(
			"expected refund status PENDING after rollback, got %s",
			refundStatus,
		)
	}

	// Order must remain CONFIRMED.
	var orderStatus string

	err = fixture.pool.QueryRow(
		ctx,
		`SELECT status
		 FROM orders
		 WHERE id = $1`,
		fixture.orderID,
	).Scan(&orderStatus)
	if err != nil {
		t.Fatalf("query order status: %v", err)
	}

	if orderStatus != "CONFIRMED" {
		t.Fatalf(
			"expected order status CONFIRMED after rollback, got %s",
			orderStatus,
		)
	}

	// No refund outbox event should exist because the transaction
	// rolled back.
	var outboxCount int

	err = fixture.pool.QueryRow(
		ctx,
		`SELECT COUNT(*)
		 FROM outbox_events
		 WHERE event_type = 'PAYMENT_REFUNDED'
		   AND payload->>'payment_id' = $1`,
		fixture.payment.ID,
	).Scan(&outboxCount)
	if err != nil {
		t.Fatalf("query refund outbox count: %v", err)
	}

	if outboxCount != 0 {
		t.Fatalf(
			"expected 0 PAYMENT_REFUNDED outbox events after rollback, got %d",
			outboxCount,
		)
	}
}

func TestCreateRefundInvalidPaymentState(t *testing.T) {
	fixture := setupRefundTest(t)
	ctx := context.Background()

	// Move payment back to PENDING for this isolated state test.
	_, err := fixture.pool.Exec(
		ctx,
		`UPDATE payments
		 SET status = 'PENDING'
		 WHERE id = $1`,
		fixture.payment.ID,
	)
	if err != nil {
		t.Fatalf("set payment pending: %v", err)
	}

	_, err = fixture.refund.CreateRefund(
		ctx,
		fixture.payment.ID,
		"refund-test-invalid-payment",
	)
	if !errors.Is(err, ErrPaymentNotRefundable) {
		t.Fatalf(
			"expected ErrPaymentNotRefundable, got %v",
			err,
		)
	}
}

func TestCreateRefundInvalidOrderState(t *testing.T) {
	fixture := setupRefundTest(t)
	ctx := context.Background()

	// The payment is SUCCEEDED, but the order is no longer
	// CONFIRMED.
	_, err := fixture.pool.Exec(
		ctx,
		`UPDATE orders
		 SET status = 'CANCELLED'
		 WHERE id = $1`,
		fixture.orderID,
	)
	if err != nil {
		t.Fatalf("set order cancelled: %v", err)
	}

	_, err = fixture.refund.CreateRefund(
		ctx,
		fixture.payment.ID,
		"refund-test-invalid-order",
	)
	if !errors.Is(err, ErrOrderNotRefundable) {
		t.Fatalf(
			"expected ErrOrderNotRefundable, got %v",
			err,
		)
	}
}

func TestCreateRefundInvalidIdempotencyKey(t *testing.T) {
	fixture := setupRefundTest(t)
	ctx := context.Background()

	_, err := fixture.refund.CreateRefund(
		ctx,
		fixture.payment.ID,
		"",
	)
	if !errors.Is(err, ErrInvalidIdempotency) {
		t.Fatalf(
			"expected ErrInvalidIdempotency, got %v",
			err,
		)
	}
}

func TestCreateRefundNotFound(t *testing.T) {
	fixture := setupRefundTest(t)
	ctx := context.Background()

	unknownPaymentID := pgtype.UUID{
		Bytes: [16]byte{
			0x01, 0x02, 0x03, 0x04,
			0x05, 0x06, 0x07, 0x08,
			0x09, 0x0a, 0x0b, 0x0c,
			0x0d, 0x0e, 0x0f, 0x10,
		},
		Valid: true,
	}

	_, err := fixture.refund.CreateRefund(
		ctx,
		unknownPaymentID,
		"refund-test-not-found",
	)
	if !errors.Is(err, ErrPaymentNotFound) {
		t.Fatalf(
			"expected ErrPaymentNotFound, got %v",
			err,
		)
	}
}
