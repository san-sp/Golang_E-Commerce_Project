package inventory

import (
	"context"
	"errors"
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

func createInventoryTestFixture(
	t *testing.T,
	ctx context.Context,
	pool *pgxpool.Pool,
	queries *db.Queries,
	quantity int64,
) uuid.UUID {
	t.Helper()

	product, err := queries.CreateProduct(
		ctx,
		db.CreateProductParams{
			Name: "Inventory Movement Test Product",
			Description: pgtype.Text{
				String: "Inventory movement test product",
				Valid:  true,
			},
		},
	)
	if err != nil {
		t.Fatalf("create test product: %v", err)
	}

	variant, err := queries.CreateProductVariant(
		ctx,
		db.CreateProductVariantParams{
			ProductID: product.ID,
			Sku:       "INV-MOVEMENT-" + uuid.NewString(),
			Price:     1000,
		},
	)
	if err != nil {
		t.Fatalf("create test variant: %v", err)
	}

	_, err = pool.Exec(
		ctx,
		`INSERT INTO inventory (variant_id, quantity)
         VALUES ($1, $2)`,
		variant.ID,
		quantity,
	)
	if err != nil {
		t.Fatalf("create test inventory: %v", err)
	}

	t.Cleanup(func() {
		_, _ = pool.Exec(
			ctx,
			`DELETE FROM inventory_movements WHERE variant_id = $1`,
			variant.ID,
		)
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

	return uuid.UUID(variant.ID.Bytes)
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
			Sku:       "INV-TEST-" + uuid.NewString(),
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

func TestAdjustInventoryRestock(t *testing.T) {
	ctx, pool, queries, cleanup := setupInventoryTest(t)
	defer cleanup()

	service := NewService(pool, queries)

	variantID := createInventoryTestFixture(
		t,
		ctx,
		pool,
		queries,
		10,
	)

	referenceType := "PURCHASE_ORDER"
	referenceID := uuid.New()

	result, err := service.AdjustInventory(
		ctx,
		variantID,
		5,
		MovementTypeRestock,
		&referenceType,
		&referenceID,
	)
	if err != nil {
		t.Fatalf("adjust inventory: %v", err)
	}

	if result.Quantity != 15 {
		t.Fatalf("expected quantity 15, got %d", result.Quantity)
	}

	if result.AvailableQuantity != 15 {
		t.Fatalf(
			"expected available quantity 15, got %d",
			result.AvailableQuantity,
		)
	}

	movements, err := queries.ListInventoryMovements(
		ctx,
		pgtype.UUID{
			Bytes: variantID,
			Valid: true,
		},
	)
	if err != nil {
		t.Fatalf("list inventory movements: %v", err)
	}

	if len(movements) != 1 {
		t.Fatalf("expected 1 movement, got %d", len(movements))
	}

	if movements[0].MovementType != string(MovementTypeRestock) {
		t.Fatalf(
			"expected movement type %s, got %s",
			MovementTypeRestock,
			movements[0].MovementType,
		)
	}

	if movements[0].Quantity != 5 {
		t.Fatalf("expected movement quantity 5, got %d", movements[0].Quantity)
	}

	if movements[0].ReferenceType.String != referenceType {
		t.Fatalf(
			"expected reference type %s, got %s",
			referenceType,
			movements[0].ReferenceType.String,
		)
	}

	if movements[0].ReferenceID.Bytes != referenceID {
		t.Fatalf("movement reference ID does not match")
	}
}

func TestAdjustInventoryDamage(t *testing.T) {
	ctx, pool, queries, cleanup := setupInventoryTest(t)
	defer cleanup()

	service := NewService(pool, queries)
	variantID := createInventoryTestFixture(t, ctx, pool, queries, 10)

	result, err := service.AdjustInventory(
		ctx,
		variantID,
		-2,
		MovementTypeDamage,
		nil,
		nil,
	)
	if err != nil {
		t.Fatalf("adjust inventory for damage: %v", err)
	}

	if result.Quantity != 8 {
		t.Errorf("expected quantity 8, got %d", result.Quantity)
	}

	if result.AvailableQuantity != 8 {
		t.Errorf("expected available quantity 8, got %d", result.AvailableQuantity)
	}

	movements, err := queries.ListInventoryMovements(
		ctx,
		pgtype.UUID{Bytes: variantID, Valid: true},
	)
	if err != nil {
		t.Fatalf("list inventory movements: %v", err)
	}

	if len(movements) != 1 {
		t.Fatalf("expected 1 movement, got %d", len(movements))
	}

	if movements[0].MovementType != string(MovementTypeDamage) {
		t.Errorf(
			"expected movement type %q, got %q",
			MovementTypeDamage,
			movements[0].MovementType,
		)
	}

	if movements[0].Quantity != -2 {
		t.Errorf("expected movement quantity -2, got %d", movements[0].Quantity)
	}
}

func TestAdjustInventoryInsufficientStock(t *testing.T) {
	ctx, pool, queries, cleanup := setupInventoryTest(t)
	defer cleanup()

	service := NewService(pool, queries)
	variantID := createInventoryTestFixture(t, ctx, pool, queries, 10)

	_, err := service.AdjustInventory(
		ctx,
		variantID,
		-11,
		MovementTypeDamage,
		nil,
		nil,
	)
	if !errors.Is(err, ErrInsufficientInventory) {
		t.Fatalf(
			"expected ErrInsufficientInventory, got %v",
			err,
		)
	}

	result, err := service.GetInventory(ctx, variantID)
	if err != nil {
		t.Fatalf("get inventory: %v", err)
	}

	if result.Quantity != 10 {
		t.Errorf("expected quantity to remain 10, got %d", result.Quantity)
	}

	movements, err := queries.ListInventoryMovements(
		ctx,
		pgtype.UUID{Bytes: variantID, Valid: true},
	)
	if err != nil {
		t.Fatalf("list inventory movements: %v", err)
	}

	if len(movements) != 0 {
		t.Errorf("expected no movement records, got %d", len(movements))
	}
}

func TestAdjustInventoryCannotReduceBelowReservedQuantity(t *testing.T) {
	ctx, pool, queries, cleanup := setupInventoryTest(t)
	defer cleanup()

	service := NewService(pool, queries)
	variantID := createInventoryTestFixture(t, ctx, pool, queries, 10)

	_, err := pool.Exec(
		ctx,
		`INSERT INTO reservations (variant_id, quantity, status, expires_at)
		 VALUES ($1, $2, $3, $4)`,
		pgtype.UUID{Bytes: variantID, Valid: true},
		int64(7),
		"ACTIVE",
		"2099-01-01T00:00:00Z",
	)
	if err != nil {
		t.Fatalf("create active reservation: %v", err)
	}

	_, err = service.AdjustInventory(
		ctx,
		variantID,
		-4,
		MovementTypeDamage,
		nil,
		nil,
	)
	if !errors.Is(err, ErrInsufficientInventory) {
		t.Fatalf(
			"expected ErrInsufficientInventory, got %v",
			err,
		)
	}

	result, err := service.GetInventory(ctx, variantID)
	if err != nil {
		t.Fatalf("get inventory: %v", err)
	}

	if result.Quantity != 10 {
		t.Errorf("expected quantity to remain 10, got %d", result.Quantity)
	}

	if result.ReservedQuantity != 7 {
		t.Errorf(
			"expected reserved quantity 7, got %d",
			result.ReservedQuantity,
		)
	}

	movements, err := queries.ListInventoryMovements(
		ctx,
		pgtype.UUID{Bytes: variantID, Valid: true},
	)
	if err != nil {
		t.Fatalf("list inventory movements: %v", err)
	}

	if len(movements) != 0 {
		t.Errorf("expected no movement records, got %d", len(movements))
	}
}

func TestAdjustInventoryRejectsInvalidMovements(t *testing.T) {
	ctx, pool, queries, cleanup := setupInventoryTest(t)
	defer cleanup()

	service := NewService(pool, queries)
	variantID := createInventoryTestFixture(t, ctx, pool, queries, 10)

	tests := []struct {
		name         string
		delta        int64
		movementType MovementType
	}{
		{
			name:         "zero quantity",
			delta:        0,
			movementType: MovementTypeRestock,
		},
		{
			name:         "negative restock",
			delta:        -1,
			movementType: MovementTypeRestock,
		},
		{
			name:         "negative return",
			delta:        -1,
			movementType: MovementTypeReturn,
		},
		{
			name:         "positive damage",
			delta:        1,
			movementType: MovementTypeDamage,
		},
		{
			name:         "unsupported movement type",
			delta:        1,
			movementType: MovementType("UNKNOWN"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := service.AdjustInventory(
				ctx,
				variantID,
				tt.delta,
				tt.movementType,
				nil,
				nil,
			)

			if !errors.Is(err, ErrInvalidMovement) {
				t.Fatalf(
					"expected ErrInvalidMovement, got %v",
					err,
				)
			}
		})
	}

	result, err := service.GetInventory(ctx, variantID)
	if err != nil {
		t.Fatalf("get inventory: %v", err)
	}

	if result.Quantity != 10 {
		t.Errorf("expected quantity to remain 10, got %d", result.Quantity)
	}

	movements, err := queries.ListInventoryMovements(
		ctx,
		pgtype.UUID{Bytes: variantID, Valid: true},
	)
	if err != nil {
		t.Fatalf("list inventory movements: %v", err)
	}

	if len(movements) != 0 {
		t.Errorf("expected no movement records, got %d", len(movements))
	}
}
