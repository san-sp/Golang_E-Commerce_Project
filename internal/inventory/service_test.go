package inventory

import (
	"context"
	"os"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/joho/godotenv"

	"github.com/san-sp/Golang_E-Commerce_Project/internal/database"
	"github.com/san-sp/Golang_E-Commerce_Project/internal/database/db"
)

func setupInventoryTest(t *testing.T) (
	context.Context,
	*pgxpool.Pool,
	*db.Queries,
	func(),
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

	queries := db.New(pool)

	cleanup := func() {
		pool.Close()
	}

	return ctx, pool, queries, cleanup
}

func TestGetInventory(t *testing.T) {
	ctx, pool, queries, cleanup := setupInventoryTest(t)
	defer cleanup()

	product, err := queries.CreateProduct(ctx, db.CreateProductParams{
		Name: "Inventory Test Product",
		Description: pgtype.Text{
			String: "Inventory test product",
			Valid:  true,
		},
	})
	if err != nil {
		t.Fatalf("create product: %v", err)
	}

	variant, err := queries.CreateProductVariant(
		ctx,
		db.CreateProductVariantParams{
			ProductID: product.ID,
			Sku:       "INV-TEST-001",
			Size: pgtype.Text{
				String: "M",
				Valid:  true,
			},
			Color: pgtype.Text{
				String: "Black",
				Valid:  true,
			},
			Price: 1000,
		},
	)
	if err != nil {
		t.Fatalf("create product variant: %v", err)
	}

	_, err = pool.Exec(
		ctx,
		`INSERT INTO inventory (variant_id, quantity)
		 VALUES ($1, $2)`,
		variant.ID,
		10,
	)
	if err != nil {
		t.Fatalf("create inventory: %v", err)
	}

	_, err = pool.Exec(
		ctx,
		`INSERT INTO reservations (
			variant_id,
			quantity,
			status,
			expires_at
		)
		VALUES ($1, $2, $3, $4)`,
		variant.ID,
		3,
		"ACTIVE",
		"2099-01-01T00:00:00Z",
	)
	if err != nil {
		t.Fatalf("create reservation: %v", err)
	}

	t.Cleanup(func() {
		_, _ = pool.Exec(
			ctx,
			`DELETE FROM reservations WHERE variant_id = $1`,
			variant.ID,
		)

		_, _ = pool.Exec(
			ctx,
			`DELETE FROM inventory WHERE variant_id = $1`,
			variant.ID,
		)

		_, _ = pool.Exec(
			ctx,
			`DELETE FROM product_variants WHERE id = $1`,
			variant.ID,
		)

		_, _ = pool.Exec(
			ctx,
			`DELETE FROM products WHERE id = $1`,
			product.ID,
		)
	})

	service := NewService(nil, queries)

	result, err := service.GetInventory(
		ctx,
		uuid.UUID(variant.ID.Bytes),
	)
	if err != nil {
		t.Fatalf("get inventory: %v", err)
	}

	if result.VariantID != uuid.UUID(variant.ID.Bytes) {
		t.Fatalf(
			"expected variant ID %v, got %v",
			variant.ID,
			result.VariantID,
		)
	}

	if result.Quantity != 10 {
		t.Fatalf(
			"expected quantity 10, got %d",
			result.Quantity,
		)
	}

	if result.ReservedQuantity != 3 {
		t.Fatalf(
			"expected reserved quantity 3, got %d",
			result.ReservedQuantity,
		)
	}

	if result.AvailableQuantity != 7 {
		t.Fatalf(
			"expected available quantity 7, got %d",
			result.AvailableQuantity,
		)
	}
}

func TestGetInventoryNotFound(t *testing.T) {
	ctx, _, queries, cleanup := setupInventoryTest(t)
	defer cleanup()

	service := NewService(nil, queries)

	_, err := service.GetInventory(ctx, uuid.New())

	if err != ErrInventoryNotFound {
		t.Fatalf(
			"expected ErrInventoryNotFound, got %v",
			err,
		)
	}
}
