package checkout

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/joho/godotenv"

	"github.com/san-sp/Golang_E-Commerce_Project/internal/database"
	"github.com/san-sp/Golang_E-Commerce_Project/internal/database/db"
	"github.com/san-sp/Golang_E-Commerce_Project/internal/order"
	"github.com/san-sp/Golang_E-Commerce_Project/internal/payment"
	"github.com/san-sp/Golang_E-Commerce_Project/internal/reservation"
)

func setupTestService(t *testing.T) (
	*Service,
	*db.Queries,
) {
	t.Helper()

	if err := godotenv.Load("../../.env"); err != nil {
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

	orderService := order.NewService(pool, queries)

	reservationService := reservation.NewService(
		pool,
		queries,
		30*time.Minute,
	)

	provider := payment.NewMockProvider()

	paymentService := payment.NewService(
		pool,
		queries,
		provider,
	)

	service := NewService(
		pool,
		queries,
		orderService,
		reservationService,
		paymentService,
	)

	return service, queries
}

func setupTestServiceWithProvider(
	t *testing.T,
) (*Service, *db.Queries, *payment.MockProvider) {
	t.Helper()

	if err := godotenv.Load("../../.env"); err != nil {
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

	orderService := order.NewService(pool, queries)

	reservationService := reservation.NewService(
		pool,
		queries,
		30*time.Minute,
	)

	provider := payment.NewMockProvider()

	paymentService := payment.NewService(
		pool,
		queries,
		provider,
	)

	service := NewService(
		pool,
		queries,
		orderService,
		reservationService,
		paymentService,
	)

	return service, queries, provider
}

func testVariantID() pgtype.UUID {
	return pgtype.UUID{
		Bytes: [16]byte{
			0xc5, 0x17, 0x4f, 0x98,
			0x5b, 0xa2, 0x45, 0x3e,
			0x92, 0xfb, 0x26, 0x6e,
			0x81, 0x8f, 0xbd, 0x92,
		},
		Valid: true,
	}
}

func TestCheckoutEmptyCart(t *testing.T) {
	service, queries := setupTestService(t)

	ctx := context.Background()

	cart, err := queries.CreateCart(ctx)
	if err != nil {
		t.Fatalf("create cart: %v", err)
	}

	t.Cleanup(func() {
		_, _ = service.pool.Exec(
			ctx,
			`DELETE FROM carts WHERE id = $1`,
			cart.ID,
		)
	})

	_, err = service.Checkout(
		ctx,
		cart.ID,
		"INR",
		"mock",
	)

	if err != ErrCartEmpty {
		t.Fatalf(
			"expected ErrCartEmpty, got %v",
			err,
		)
	}
}

func TestCheckoutSuccess(t *testing.T) {
	service, queries := setupTestService(t)

	ctx := context.Background()
	variantID := testVariantID()

	cart, err := queries.CreateCart(ctx)
	if err != nil {
		t.Fatalf("create cart: %v", err)
	}

	t.Cleanup(func() {
		_, _ = service.pool.Exec(
			ctx,
			`DELETE FROM carts WHERE id = $1`,
			cart.ID,
		)
	})

	_, err = queries.AddCartItem(
		ctx,
		db.AddCartItemParams{
			CartID:    cart.ID,
			VariantID: variantID,
			Quantity:  2,
		},
	)
	if err != nil {
		t.Fatalf("add cart item: %v", err)
	}

	result, err := service.Checkout(
		ctx,
		cart.ID,
		"INR",
		"mock",
	)
	if err != nil {
		t.Fatalf("checkout: %v", err)
	}

	t.Cleanup(func() {
		_, _ = service.pool.Exec(
			ctx,
			`DELETE FROM payments WHERE order_id = $1`,
			result.Order.ID,
		)

		_, _ = service.pool.Exec(
			ctx,
			`DELETE FROM reservations WHERE order_id = $1`,
			result.Order.ID,
		)

		_, _ = service.pool.Exec(
			ctx,
			`DELETE FROM orders WHERE id = $1`,
			result.Order.ID,
		)
	})

	if !result.Order.ID.Valid {
		t.Fatal("expected order ID to be valid")
	}

	if result.Order.Status != "PENDING" {
		t.Fatalf(
			"expected order status PENDING, got %s",
			result.Order.Status,
		)
	}

	if result.Order.Currency != "INR" {
		t.Fatalf(
			"expected currency INR, got %s",
			result.Order.Currency,
		)
	}

	if result.Order.TotalAmount != 1799800 {
		t.Fatalf(
			"expected total 1799800, got %d",
			result.Order.TotalAmount,
		)
	}

	if len(result.OrderItems) != 1 {
		t.Fatalf(
			"expected 1 order item, got %d",
			len(result.OrderItems),
		)
	}

	if result.OrderItems[0].Quantity != 2 {
		t.Fatalf(
			"expected quantity 2, got %d",
			result.OrderItems[0].Quantity,
		)
	}

	if result.OrderItems[0].UnitPrice != 899900 {
		t.Fatalf(
			"expected unit price 899900, got %d",
			result.OrderItems[0].UnitPrice,
		)
	}

	if len(result.Reservations) != 1 {
		t.Fatalf(
			"expected 1 reservation, got %d",
			len(result.Reservations),
		)
	}

	if result.Reservations[0].Status != "ACTIVE" {
		t.Fatalf(
			"expected reservation status ACTIVE, got %s",
			result.Reservations[0].Status,
		)
	}

	if result.Reservations[0].OrderID != result.Order.ID {
		t.Fatal("expected reservation to reference created order")
	}

	cartAfterCheckout, err := queries.GetCart(ctx, cart.ID)
	if err != nil {
		t.Fatalf("get cart after checkout: %v", err)
	}

	if cartAfterCheckout.Status != "CONVERTED" {
		t.Fatalf(
			"expected cart status CONVERTED, got %s",
			cartAfterCheckout.Status,
		)
	}

}

func TestCheckoutRollsBackOrderWhenReservationFails(t *testing.T) {
	service, queries := setupTestService(t)

	ctx := context.Background()
	variantID := testVariantID()

	cart, err := queries.CreateCart(ctx)
	if err != nil {
		t.Fatalf("create cart: %v", err)
	}

	t.Cleanup(func() {
		_, _ = service.pool.Exec(
			ctx,
			`DELETE FROM carts WHERE id = $1`,
			cart.ID,
		)
	})

	_, err = queries.AddCartItem(
		ctx,
		db.AddCartItemParams{
			CartID:    cart.ID,
			VariantID: variantID,
			Quantity:  999999999,
		},
	)
	if err != nil {
		t.Fatalf("add cart item: %v", err)
	}

	currency := "TEST-ROLLBACK"

	_, err = service.Checkout(
		ctx,
		cart.ID,
		currency,
		"mock",
	)
	if err == nil {
		t.Fatal("expected checkout to fail")
	}

	var orderCount int64

	err = service.pool.QueryRow(
		ctx,
		`SELECT COUNT(*)
		 FROM orders
		 WHERE currency = $1`,
		currency,
	).Scan(&orderCount)
	if err != nil {
		t.Fatalf("count orders: %v", err)
	}

	if orderCount != 0 {
		t.Fatalf(
			"expected 0 orders after rollback, got %d",
			orderCount,
		)
	}
}

func TestCheckoutPaymentProviderFailure(t *testing.T) {
	service, queries, provider := setupTestServiceWithProvider(t)

	ctx := context.Background()
	variantID := testVariantID()

	cart, err := queries.CreateCart(ctx)
	if err != nil {
		t.Fatalf("create cart: %v", err)
	}

	t.Cleanup(func() {
		_, _ = service.pool.Exec(
			ctx,
			`DELETE FROM carts WHERE id = $1`,
			cart.ID,
		)
	})

	_, err = queries.AddCartItem(
		ctx,
		db.AddCartItemParams{
			CartID:    cart.ID,
			VariantID: variantID,
			Quantity:  1,
		},
	)
	if err != nil {
		t.Fatalf("add cart item: %v", err)
	}

	provider.SetFailure(true)

	result, err := service.Checkout(
		ctx,
		cart.ID,
		"INR",
		"mock",
	)

	if err == nil {
		t.Fatal("expected checkout to fail")
	}

	if result.Payment.Status != "FAILED" {
		t.Fatalf(
			"expected payment status FAILED, got %s",
			result.Payment.Status,
		)
	}

	var orderStatus string

	err = service.pool.QueryRow(
		ctx,
		`SELECT status
		 FROM orders
		 WHERE id = $1`,
		result.Order.ID,
	).Scan(&orderStatus)
	if err != nil {
		t.Fatalf("get order status: %v", err)
	}

	if orderStatus != "CANCELLED" {
		t.Fatalf(
			"expected order status CANCELLED, got %s",
			orderStatus,
		)
	}

	var reservationStatus string

	err = service.pool.QueryRow(
		ctx,
		`SELECT status
		 FROM reservations
		 WHERE order_id = $1`,
		result.Order.ID,
	).Scan(&reservationStatus)
	if err != nil {
		t.Fatalf("get reservation status: %v", err)
	}

	if reservationStatus != "CANCELLED" {
		t.Fatalf(
			"expected reservation status CANCELLED, got %s",
			reservationStatus,
		)
	}

	t.Cleanup(func() {
		_, _ = service.pool.Exec(
			ctx,
			`DELETE FROM payments WHERE order_id = $1`,
			result.Order.ID,
		)

		_, _ = service.pool.Exec(
			ctx,
			`DELETE FROM reservations WHERE order_id = $1`,
			result.Order.ID,
		)

		_, _ = service.pool.Exec(
			ctx,
			`DELETE FROM orders WHERE id = $1`,
			result.Order.ID,
		)
	})
}

func TestCheckoutPaymentProviderUnknown(t *testing.T) {
	service, queries, provider := setupTestServiceWithProvider(t)

	ctx := context.Background()
	variantID := testVariantID()

	cart, err := queries.CreateCart(ctx)
	if err != nil {
		t.Fatalf("create cart: %v", err)
	}

	t.Cleanup(func() {
		_, _ = service.pool.Exec(
			ctx,
			`DELETE FROM carts WHERE id = $1`,
			cart.ID,
		)
	})

	_, err = queries.AddCartItem(
		ctx,
		db.AddCartItemParams{
			CartID:    cart.ID,
			VariantID: variantID,
			Quantity:  1,
		},
	)
	if err != nil {
		t.Fatalf("add cart item: %v", err)
	}

	provider.SetUnknown()

	result, err := service.Checkout(
		ctx,
		cart.ID,
		"INR",
		"mock",
	)

	if err == nil {
		t.Fatal("expected checkout to return unknown payment outcome")
	}

	if result.Payment.Status != "PENDING" {
		t.Fatalf(
			"expected payment status PENDING, got %s",
			result.Payment.Status,
		)
	}

	var orderStatus string

	err = service.pool.QueryRow(
		ctx,
		`SELECT status
		 FROM orders
		 WHERE id = $1`,
		result.Order.ID,
	).Scan(&orderStatus)
	if err != nil {
		t.Fatalf("get order status: %v", err)
	}

	if orderStatus != "PENDING" {
		t.Fatalf(
			"expected order status PENDING, got %s",
			orderStatus,
		)
	}

	var reservationStatus string

	err = service.pool.QueryRow(
		ctx,
		`SELECT status
		 FROM reservations
		 WHERE order_id = $1`,
		result.Order.ID,
	).Scan(&reservationStatus)
	if err != nil {
		t.Fatalf("get reservation status: %v", err)
	}

	if reservationStatus != "ACTIVE" {
		t.Fatalf(
			"expected reservation status ACTIVE, got %s",
			reservationStatus,
		)
	}

	t.Cleanup(func() {
		_, _ = service.pool.Exec(
			ctx,
			`DELETE FROM payments WHERE order_id = $1`,
			result.Order.ID,
		)

		_, _ = service.pool.Exec(
			ctx,
			`DELETE FROM reservations WHERE order_id = $1`,
			result.Order.ID,
		)

		_, _ = service.pool.Exec(
			ctx,
			`DELETE FROM orders WHERE id = $1`,
			result.Order.ID,
		)
	})
}

func TestCancelOrder(t *testing.T) {
	service, queries := setupTestService(t)

	ctx := context.Background()
	variantID := testVariantID()

	cart, err := queries.CreateCart(ctx)
	if err != nil {
		t.Fatalf("create cart: %v", err)
	}

	t.Cleanup(func() {
		_, _ = service.pool.Exec(
			ctx,
			`DELETE FROM carts WHERE id = $1`,
			cart.ID,
		)
	})

	_, err = queries.AddCartItem(
		ctx,
		db.AddCartItemParams{
			CartID:    cart.ID,
			VariantID: variantID,
			Quantity:  1,
		},
	)
	if err != nil {
		t.Fatalf("add cart item: %v", err)
	}

	result, err := service.Checkout(
		ctx,
		cart.ID,
		"INR",
		"mock",
	)
	if err != nil {
		t.Fatalf("checkout: %v", err)
	}

	t.Cleanup(func() {
		_, _ = service.pool.Exec(
			ctx,
			`DELETE FROM orders WHERE id = $1`,
			result.Order.ID,
		)
	})

	cancelledOrder, reservations, err := service.CancelOrder(
		ctx,
		result.Order.ID,
	)
	if err != nil {
		t.Fatalf("cancel order: %v", err)
	}

	if cancelledOrder.Status != "CANCELLED" {
		t.Fatalf(
			"expected order status CANCELLED, got %s",
			cancelledOrder.Status,
		)
	}

	if len(reservations) != 1 {
		t.Fatalf(
			"expected 1 cancelled reservation, got %d",
			len(reservations),
		)
	}

	if reservations[0].Status != "CANCELLED" {
		t.Fatalf(
			"expected reservation status CANCELLED, got %s",
			reservations[0].Status,
		)
	}
}

func TestCancelConfirmedOrder(t *testing.T) {
	service, queries := setupTestService(t)

	ctx := context.Background()

	orderRecord, err := queries.CreateOrder(
		ctx,
		db.CreateOrderParams{
			TotalAmount: 10000,
			Currency:    "INR",
		},
	)
	if err != nil {
		t.Fatalf("create order: %v", err)
	}

	t.Cleanup(func() {
		_, _ = service.pool.Exec(
			ctx,
			`DELETE FROM orders WHERE id = $1`,
			orderRecord.ID,
		)
	})

	_, err = queries.ConfirmOrder(
		ctx,
		orderRecord.ID,
	)
	if err != nil {
		t.Fatalf("confirm order: %v", err)
	}

	_, _, err = service.CancelOrder(
		ctx,
		orderRecord.ID,
	)

	if !errors.Is(err, order.ErrInvalidOrderState) {
		t.Fatalf(
			"expected ErrInvalidOrderState, got %v",
			err,
		)
	}
}

func TestCancelOrderNotFound(t *testing.T) {
	service, _ := setupTestService(t)

	ctx := context.Background()

	orderID := pgtype.UUID{
		Bytes: uuid.New(),
		Valid: true,
	}

	_, _, err := service.CancelOrder(
		ctx,
		orderID,
	)

	if !errors.Is(err, order.ErrOrderNotFound) {
		t.Fatalf(
			"expected ErrOrderNotFound, got %v",
			err,
		)
	}
}
