package cart

import (
	"context"
	"os"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/joho/godotenv"

	"github.com/san-sp/Golang_E-Commerce_Project/internal/database"
	"github.com/san-sp/Golang_E-Commerce_Project/internal/database/db"
)

func setupCartTest(t *testing.T) (
	context.Context,
	*db.Queries,
	pgtype.UUID,
	func(),
) {
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

	product, err := queries.CreateProduct(
		ctx,
		db.CreateProductParams{
			Name: "Cart Test Product",
			Description: pgtype.Text{
				String: "Cart test product",
				Valid:  true,
			},
		},
	)
	if err != nil {
		pool.Close()
		t.Fatalf("create test product: %v", err)
	}

	variant, err := queries.CreateProductVariant(
		ctx,
		db.CreateProductVariantParams{
			ProductID: product.ID,
			Sku:       "CART-TEST-" + uuid.NewString(),
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
		pool.Close()
		t.Fatalf("create test product variant: %v", err)
	}

	cleanup := func() {
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

		pool.Close()
	}

	return ctx, queries, variant.ID, cleanup
}

func TestCreateCart(t *testing.T) {
	ctx, queries, _, cleanup := setupCartTest(t)
	defer cleanup()

	service := NewService(queries)

	cart, err := service.CreateCart(ctx)
	if err != nil {
		t.Fatalf("create cart: %v", err)
	}

	if !cart.ID.Valid {
		t.Fatal("expected cart ID to be valid")
	}

	if cart.Status != "ACTIVE" {
		t.Fatalf(
			"expected cart status ACTIVE, got %s",
			cart.Status,
		)
	}

	t.Cleanup(func() {
		_, err := queries.GetCart(ctx, cart.ID)
		if err == nil {
			_, _ = queries.ConvertCart(ctx, cart.ID)
		}
	})
}

func TestAddItemIncrementsExistingQuantity(t *testing.T) {
	ctx, queries, variantID, cleanup := setupCartTest(t)
	defer cleanup()

	service := NewService(queries)

	cart, err := service.CreateCart(ctx)
	if err != nil {
		t.Fatalf("create cart: %v", err)
	}

	t.Cleanup(func() {
		_ = queries.RemoveCartItem(
			ctx,
			db.RemoveCartItemParams{
				CartID:    cart.ID,
				VariantID: variantID,
			},
		)

		_, _ = queries.ConvertCart(ctx, cart.ID)
	})

	firstItem, err := service.AddItem(
		ctx,
		cart.ID,
		variantID,
		2,
	)
	if err != nil {
		t.Fatalf("add first item: %v", err)
	}

	if firstItem.Quantity != 2 {
		t.Fatalf(
			"expected quantity 2, got %d",
			firstItem.Quantity,
		)
	}

	secondItem, err := service.AddItem(
		ctx,
		cart.ID,
		variantID,
		3,
	)
	if err != nil {
		t.Fatalf("add second item: %v", err)
	}

	if secondItem.Quantity != 5 {
		t.Fatalf(
			"expected quantity 5, got %d",
			secondItem.Quantity,
		)
	}
}

func TestAddItemRejectsInvalidQuantity(t *testing.T) {
	ctx, queries, variantID, cleanup := setupCartTest(t)
	defer cleanup()

	service := NewService(queries)

	cart, err := service.CreateCart(ctx)
	if err != nil {
		t.Fatalf("create cart: %v", err)
	}

	t.Cleanup(func() {
		_, _ = queries.ConvertCart(ctx, cart.ID)
	})

	_, err = service.AddItem(
		ctx,
		cart.ID,
		variantID,
		0,
	)

	if err != ErrInvalidQuantity {
		t.Fatalf(
			"expected ErrInvalidQuantity, got %v",
			err,
		)
	}
}

func TestAddItemRejectsConvertedCart(t *testing.T) {
	ctx, queries, variantID, cleanup := setupCartTest(t)
	defer cleanup()

	service := NewService(queries)

	cart, err := service.CreateCart(ctx)
	if err != nil {
		t.Fatalf("create cart: %v", err)
	}

	_, err = queries.ConvertCart(ctx, cart.ID)
	if err != nil {
		t.Fatalf("convert cart: %v", err)
	}

	_, err = service.AddItem(
		ctx,
		cart.ID,
		variantID,
		1,
	)

	if err != ErrCartNotActive {
		t.Fatalf(
			"expected ErrCartNotActive, got %v",
			err,
		)
	}
}

func TestUpdateItemQuantity(t *testing.T) {
	ctx, queries, variantID, cleanup := setupCartTest(t)
	defer cleanup()

	service := NewService(queries)

	cart, err := service.CreateCart(ctx)
	if err != nil {
		t.Fatalf("create cart: %v", err)
	}

	t.Cleanup(func() {
		_ = queries.RemoveCartItem(
			ctx,
			db.RemoveCartItemParams{
				CartID:    cart.ID,
				VariantID: variantID,
			},
		)

		_, _ = queries.ConvertCart(ctx, cart.ID)
	})

	_, err = service.AddItem(
		ctx,
		cart.ID,
		variantID,
		2,
	)
	if err != nil {
		t.Fatalf("add item: %v", err)
	}

	item, err := service.UpdateItemQuantity(
		ctx,
		cart.ID,
		variantID,
		7,
	)
	if err != nil {
		t.Fatalf("update item quantity: %v", err)
	}

	if item.Quantity != 7 {
		t.Fatalf(
			"expected quantity 7, got %d",
			item.Quantity,
		)
	}
}

func TestRemoveItem(t *testing.T) {
	ctx, queries, variantID, cleanup := setupCartTest(t)
	defer cleanup()

	service := NewService(queries)

	cart, err := service.CreateCart(ctx)
	if err != nil {
		t.Fatalf("create cart: %v", err)
	}

	t.Cleanup(func() {
		_ = queries.RemoveCartItem(
			ctx,
			db.RemoveCartItemParams{
				CartID:    cart.ID,
				VariantID: variantID,
			},
		)

		_, _ = queries.ConvertCart(ctx, cart.ID)
	})

	_, err = service.AddItem(
		ctx,
		cart.ID,
		variantID,
		2,
	)
	if err != nil {
		t.Fatalf("add item: %v", err)
	}

	err = service.RemoveItem(
		ctx,
		cart.ID,
		variantID,
	)
	if err != nil {
		t.Fatalf("remove item: %v", err)
	}

	_, err = queries.GetCartItem(
		ctx,
		db.GetCartItemParams{
			CartID:    cart.ID,
			VariantID: variantID,
		},
	)

	if err == nil {
		t.Fatal("expected cart item to be removed")
	}
}

func TestGetCartItems(t *testing.T) {
	ctx, queries, variantID, cleanup := setupCartTest(t)
	defer cleanup()

	service := NewService(queries)

	cart, err := service.CreateCart(ctx)
	if err != nil {
		t.Fatalf("create cart: %v", err)
	}

	t.Cleanup(func() {
		_ = queries.RemoveCartItem(
			ctx,
			db.RemoveCartItemParams{
				CartID:    cart.ID,
				VariantID: variantID,
			},
		)

		_, _ = queries.ConvertCart(ctx, cart.ID)
	})

	_, err = service.AddItem(
		ctx,
		cart.ID,
		variantID,
		4,
	)
	if err != nil {
		t.Fatalf("add item: %v", err)
	}

	items, err := service.GetCartItems(
		ctx,
		cart.ID,
	)
	if err != nil {
		t.Fatalf("get cart items: %v", err)
	}

	if len(items) != 1 {
		t.Fatalf(
			"expected 1 cart item, got %d",
			len(items),
		)
	}

	if items[0].Quantity != 4 {
		t.Fatalf(
			"expected quantity 4, got %d",
			items[0].Quantity,
		)
	}
}

func TestGetCartNotFound(t *testing.T) {
	ctx, queries, _, cleanup := setupCartTest(t)
	defer cleanup()

	service := NewService(queries)

	missingCartID := pgtype.UUID{
		Bytes: uuid.New(),
		Valid: true,
	}

	_, err := service.GetCart(
		ctx,
		missingCartID,
	)

	if err != ErrCartNotFound {
		t.Fatalf(
			"expected ErrCartNotFound, got %v",
			err,
		)
	}
}
