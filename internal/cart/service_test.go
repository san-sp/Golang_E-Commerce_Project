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

var testVariantID = pgtype.UUID{
	Bytes: [16]byte{
		0xc5, 0x17, 0x4f, 0x98,
		0x5b, 0xa2, 0x45, 0x3e,
		0x92, 0xfb, 0x26, 0x6e,
		0x81, 0x8f, 0xbd, 0x92,
	},
	Valid: true,
}

func setupCartTest(t *testing.T) (
	context.Context,
	*db.Queries,
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

	cleanup := func() {
		pool.Close()
	}

	return ctx, queries, cleanup
}

func TestCreateCart(t *testing.T) {
	ctx, queries, cleanup := setupCartTest(t)
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
	ctx, queries, cleanup := setupCartTest(t)
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
				VariantID: testVariantID,
			},
		)

		_, _ = queries.ConvertCart(ctx, cart.ID)
	})

	firstItem, err := service.AddItem(
		ctx,
		cart.ID,
		testVariantID,
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
		testVariantID,
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
	ctx, queries, cleanup := setupCartTest(t)
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
		testVariantID,
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
	ctx, queries, cleanup := setupCartTest(t)
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
		testVariantID,
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
	ctx, queries, cleanup := setupCartTest(t)
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
				VariantID: testVariantID,
			},
		)

		_, _ = queries.ConvertCart(ctx, cart.ID)
	})

	_, err = service.AddItem(
		ctx,
		cart.ID,
		testVariantID,
		2,
	)
	if err != nil {
		t.Fatalf("add item: %v", err)
	}

	item, err := service.UpdateItemQuantity(
		ctx,
		cart.ID,
		testVariantID,
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
	ctx, queries, cleanup := setupCartTest(t)
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
				VariantID: testVariantID,
			},
		)

		_, _ = queries.ConvertCart(ctx, cart.ID)
	})

	_, err = service.AddItem(
		ctx,
		cart.ID,
		testVariantID,
		2,
	)
	if err != nil {
		t.Fatalf("add item: %v", err)
	}

	err = service.RemoveItem(
		ctx,
		cart.ID,
		testVariantID,
	)
	if err != nil {
		t.Fatalf("remove item: %v", err)
	}

	_, err = queries.GetCartItem(
		ctx,
		db.GetCartItemParams{
			CartID:    cart.ID,
			VariantID: testVariantID,
		},
	)

	if err == nil {
		t.Fatal("expected cart item to be removed")
	}
}

func TestGetCartItems(t *testing.T) {
	ctx, queries, cleanup := setupCartTest(t)
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
				VariantID: testVariantID,
			},
		)

		_, _ = queries.ConvertCart(ctx, cart.ID)
	})

	_, err = service.AddItem(
		ctx,
		cart.ID,
		testVariantID,
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
	ctx, queries, cleanup := setupCartTest(t)
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
