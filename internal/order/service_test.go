package order

import (
	"context"
	"errors"
	"os"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/joho/godotenv"

	"github.com/san-sp/Golang_E-Commerce_Project/internal/database"
	"github.com/san-sp/Golang_E-Commerce_Project/internal/database/db"
)

func createOrderTestVariant(t *testing.T, service *Service) pgtype.UUID {
	t.Helper()

	ctx := context.Background()

	var productID pgtype.UUID
	err := service.pool.QueryRow(
		ctx,
		`INSERT INTO products (name, description)
         VALUES ('Order Test Product', 'Test product')
         RETURNING id`,
	).Scan(&productID)
	if err != nil {
		t.Fatalf("create test product: %v", err)
	}

	var variantID pgtype.UUID
	err = service.pool.QueryRow(
		ctx,
		`INSERT INTO product_variants (
            product_id,
            sku,
            price
         )
         VALUES (
            $1,
            'order-test-' || gen_random_uuid()::text,
            899900
         )
         RETURNING id`,
		productID,
	).Scan(&variantID)
	if err != nil {
		t.Fatalf("create test variant: %v", err)
	}

	t.Cleanup(func() {
		_, _ = service.pool.Exec(
			ctx,
			`DELETE FROM product_variants WHERE id = $1`,
			variantID,
		)

		_, _ = service.pool.Exec(
			ctx,
			`DELETE FROM products WHERE id = $1`,
			productID,
		)
	})

	return variantID
}

func setupTestService(t *testing.T) (*Service, *db.Queries) {
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

	t.Cleanup(func() {
		pool.Close()
	})

	queries := db.New(pool)

	return NewService(pool, queries), queries
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

func TestCreateOrder(t *testing.T) {
	service, queries := setupTestService(t)

	ctx := context.Background()

	variantID := createOrderTestVariant(t, service)

	order, items, err := service.CreateOrder(
		ctx,
		[]ItemInput{
			{
				VariantID: variantID,
				Quantity:  2,
				UnitPrice: 899900,
			},
			{
				VariantID: variantID,
				Quantity:  1,
				UnitPrice: 50000,
			},
		},
		"INR",
	)
	if err != nil {
		t.Fatalf("create order: %v", err)
	}

	t.Cleanup(func() {
		_, err := queries.GetOrder(ctx, order.ID)
		if err == nil {
			_, _ = service.pool.Exec(
				ctx,
				`DELETE FROM orders WHERE id = $1`,
				order.ID,
			)
		}
	})

	if !order.ID.Valid {
		t.Fatal("expected order ID to be valid")
	}

	if order.Status != "PENDING" {
		t.Fatalf("expected status PENDING, got %s", order.Status)
	}

	expectedTotal := int64(2*899900 + 50000)

	if order.TotalAmount != expectedTotal {
		t.Fatalf(
			"expected total %d, got %d",
			expectedTotal,
			order.TotalAmount,
		)
	}

	if order.Currency != "INR" {
		t.Fatalf("expected currency INR, got %s", order.Currency)
	}

	if len(items) != 2 {
		t.Fatalf("expected 2 order items, got %d", len(items))
	}
}

func TestCreateOrderRejectsEmptyItems(t *testing.T) {
	service, _ := setupTestService(t)

	_, _, err := service.CreateOrder(
		context.Background(),
		nil,
		"INR",
	)

	if err == nil {
		t.Fatal("expected error for empty order")
	}
}

func TestCreateOrderRejectsInvalidQuantity(t *testing.T) {
	service, _ := setupTestService(t)

	_, _, err := service.CreateOrder(
		context.Background(),
		[]ItemInput{
			{
				VariantID: testVariantID(),
				Quantity:  0,
				UnitPrice: 899900,
			},
		},
		"INR",
	)

	if err == nil {
		t.Fatal("expected error for invalid quantity")
	}
}

func TestCreateOrderRejectsNegativePrice(t *testing.T) {
	service, _ := setupTestService(t)

	_, _, err := service.CreateOrder(
		context.Background(),
		[]ItemInput{
			{
				VariantID: testVariantID(),
				Quantity:  1,
				UnitPrice: -1,
			},
		},
		"INR",
	)

	if err == nil {
		t.Fatal("expected error for negative price")
	}
}

func TestGetOrder(t *testing.T) {
	service, _ := setupTestService(t)

	ctx := context.Background()
	variantID := createOrderTestVariant(t, service)

	createdOrder, _, err := service.CreateOrder(
		ctx,
		[]ItemInput{
			{
				VariantID: variantID,
				Quantity:  2,
				UnitPrice: 899900,
			},
		},
		"INR",
	)
	if err != nil {
		t.Fatalf("create order: %v", err)
	}

	t.Cleanup(func() {
		_, _ = service.pool.Exec(
			ctx,
			`DELETE FROM orders WHERE id = $1`,
			createdOrder.ID,
		)
	})

	orderID := uuid.UUID(createdOrder.ID.Bytes)

	fetchedOrder, items, err := service.GetOrder(ctx, orderID)
	if err != nil {
		t.Fatalf("get order: %v", err)
	}

	if fetchedOrder.ID != createdOrder.ID {
		t.Fatal("fetched order ID does not match created order")
	}

	if fetchedOrder.TotalAmount != 1799800 {
		t.Fatalf(
			"expected total 1799800, got %d",
			fetchedOrder.TotalAmount,
		)
	}

	if len(items) != 1 {
		t.Fatalf("expected 1 order item, got %d", len(items))
	}

	if items[0].Quantity != 2 {
		t.Fatalf(
			"expected quantity 2, got %d",
			items[0].Quantity,
		)
	}
}

func TestConfirmOrder(t *testing.T) {
	service, queries := setupTestService(t)

	ctx := context.Background()

	order, err := queries.CreateOrder(
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
			order.ID,
		)
	})

	confirmed, err := service.ConfirmOrder(
		ctx,
		order.ID,
	)
	if err != nil {
		t.Fatalf("confirm order: %v", err)
	}

	if confirmed.Status != "CONFIRMED" {
		t.Fatalf(
			"expected status CONFIRMED, got %s",
			confirmed.Status,
		)
	}
}

func TestCancelOrder(t *testing.T) {
	service, queries := setupTestService(t)

	ctx := context.Background()

	order, err := queries.CreateOrder(
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
			order.ID,
		)
	})

	cancelled, err := service.CancelOrder(
		ctx,
		order.ID,
	)
	if err != nil {
		t.Fatalf("cancel order: %v", err)
	}

	if cancelled.Status != "CANCELLED" {
		t.Fatalf(
			"expected status CANCELLED, got %s",
			cancelled.Status,
		)
	}
}

func TestConfirmCancelledOrderFails(t *testing.T) {
	service, queries := setupTestService(t)

	ctx := context.Background()

	order, err := queries.CreateOrder(
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
			order.ID,
		)
	})

	_, err = service.CancelOrder(ctx, order.ID)
	if err != nil {
		t.Fatalf("cancel order: %v", err)
	}

	_, err = service.ConfirmOrder(ctx, order.ID)

	if !errors.Is(err, ErrInvalidOrderState) {
		t.Fatalf(
			"expected ErrInvalidOrderState, got %v",
			err,
		)
	}
}

func TestConfirmOrderNotFound(t *testing.T) {
	service, _ := setupTestService(t)

	ctx := context.Background()

	orderID := pgtype.UUID{
		Bytes: uuid.New(),
		Valid: true,
	}

	_, err := service.ConfirmOrder(
		ctx,
		orderID,
	)

	if !errors.Is(err, ErrOrderNotFound) {
		t.Fatalf(
			"expected ErrOrderNotFound, got %v",
			err,
		)
	}
}
