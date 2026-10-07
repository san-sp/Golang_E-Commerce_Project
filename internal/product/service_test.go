package product

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

func setupProductTest(t *testing.T) (
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

func TestGetProduct(t *testing.T) {
	ctx, pool, queries, cleanup := setupProductTest(t)
	defer cleanup()

	product, err := queries.CreateProduct(ctx, db.CreateProductParams{
		Name: "Test Product",
		Description: pgtype.Text{
			String: "Test product description",
			Valid:  true,
		},
	})
	if err != nil {
		t.Fatalf("create product: %v", err)
	}

	productID := uuid.UUID(product.ID.Bytes)

	t.Cleanup(func() {
		_, _ = queries.GetProduct(ctx, product.ID)
	})

	service := NewService(nil, queries)

	result, err := service.GetProduct(ctx, productID)
	if err != nil {
		t.Fatalf("get product: %v", err)
	}

	if result.ID != product.ID {
		t.Fatalf(
			"expected product ID %v, got %v",
			product.ID,
			result.ID,
		)
	}

	if result.Name != "Test Product" {
		t.Fatalf(
			"expected product name Test Product, got %s",
			result.Name,
		)
	}

	if result.Description.String != "Test product description" {
		t.Fatalf(
			"expected product description %q, got %q",
			"Test product description",
			result.Description.String,
		)
	}

	// Clean up the test record.
	t.Cleanup(func() {
		_, _ = pool.Exec(
			ctx,
			`DELETE FROM products WHERE id = $1`,
			product.ID,
		)
	})

	// No DELETE query is needed in the application query layer.
	_, _ = queries.GetProduct(ctx, product.ID)
}

func TestGetProductNotFound(t *testing.T) {
	ctx, _, queries, cleanup := setupProductTest(t)
	defer cleanup()

	service := NewService(nil, queries)

	_, err := service.GetProduct(ctx, uuid.New())
	if err != ErrProductNotFound {
		t.Fatalf(
			"expected ErrProductNotFound, got %v",
			err,
		)
	}
}

func TestListProducts(t *testing.T) {
	ctx, pool, queries, cleanup := setupProductTest(t)
	defer cleanup()

	firstProduct, err := queries.CreateProduct(
		ctx,
		db.CreateProductParams{
			Name: "Product A",
		},
	)
	if err != nil {
		t.Fatalf("create first product: %v", err)
	}

	secondProduct, err := queries.CreateProduct(
		ctx,
		db.CreateProductParams{
			Name: "Product B",
		},
	)
	if err != nil {
		t.Fatalf("create second product: %v", err)
	}

	t.Cleanup(func() {
		_, _ = pool.Exec(
			ctx,
			`DELETE FROM products WHERE id IN ($1, $2)`,
			firstProduct.ID,
			secondProduct.ID,
		)
	})

	service := NewService(pool, queries)

	products, err := service.ListProducts(ctx, 1, 20)
	if err != nil {
		t.Fatalf("list products: %v", err)
	}

	if len(products) < 2 {
		t.Fatalf(
			"expected at least 2 products, got %d",
			len(products),
		)
	}
}

func TestListProductsRejectsInvalidPage(t *testing.T) {
	ctx, _, queries, cleanup := setupProductTest(t)
	defer cleanup()

	service := NewService(nil, queries)

	_, err := service.ListProducts(ctx, 0, 20)
	if err != ErrInvalidPage {
		t.Fatalf(
			"expected ErrInvalidPage, got %v",
			err,
		)
	}
}

func TestListProductsRejectsInvalidLimit(t *testing.T) {
	ctx, _, queries, cleanup := setupProductTest(t)
	defer cleanup()

	service := NewService(nil, queries)

	_, err := service.ListProducts(ctx, 1, 101)
	if err != ErrInvalidLimit {
		t.Fatalf(
			"expected ErrInvalidLimit, got %v",
			err,
		)
	}
}

func TestListProductsRejectsZeroLimit(t *testing.T) {
	ctx, _, queries, cleanup := setupProductTest(t)
	defer cleanup()

	service := NewService(nil, queries)

	_, err := service.ListProducts(ctx, 1, 0)
	if err != ErrInvalidLimit {
		t.Fatalf(
			"expected ErrInvalidLimit, got %v",
			err,
		)
	}
}

func TestGetProductVariant(t *testing.T) {
	ctx, pool, queries, cleanup := setupProductTest(t)
	defer cleanup()

	product, err := queries.CreateProduct(ctx, db.CreateProductParams{
		Name: "Test Product",
		Description: pgtype.Text{
			String: "Test product description",
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
			Sku:       "TEST-SKU-" + uuid.NewString(),
			Size: pgtype.Text{
				Valid: false,
			},
			Color: pgtype.Text{
				Valid: false,
			},
			Price: 99900,
		},
	)
	if err != nil {
		t.Fatalf("create product variant: %v", err)
	}

	service := NewService(nil, queries)

	result, err := service.GetProductVariant(
		ctx,
		uuid.UUID(variant.ID.Bytes),
	)
	if err != nil {
		t.Fatalf("get product variant: %v", err)
	}

	if result.ID != variant.ID {
		t.Fatalf(
			"expected variant ID %v, got %v",
			variant.ID,
			result.ID,
		)
	}

	if result.Sku != variant.Sku {
		t.Fatalf(
			"expected SKU %s, got %s",
			variant.Sku,
			result.Sku,
		)
	}

	if result.Price != 99900 {
		t.Fatalf(
			"expected price 99900, got %d",
			result.Price,
		)
	}

	t.Cleanup(func() {
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
}

func TestGetProductVariantNotFound(t *testing.T) {
	ctx, _, queries, cleanup := setupProductTest(t)
	defer cleanup()

	service := NewService(nil, queries)

	_, err := service.GetProductVariant(ctx, uuid.New())
	if err != ErrProductVariantNotFound {
		t.Fatalf(
			"expected ErrProductVariantNotFound, got %v",
			err,
		)
	}
}

func TestListProductVariants(t *testing.T) {
	ctx, pool, queries, cleanup := setupProductTest(t)
	defer cleanup()

	product, err := queries.CreateProduct(
		ctx,
		db.CreateProductParams{
			Name: "Variant Test Product",
		},
	)
	if err != nil {
		t.Fatalf("create product: %v", err)
	}

	variantA, err := queries.CreateProductVariant(
		ctx,
		db.CreateProductVariantParams{
			ProductID: product.ID,
			Sku:       "VARIANT-A-" + uuid.NewString(),
			Size: pgtype.Text{
				String: "M",
				Valid:  true,
			},
			Color: pgtype.Text{
				String: "Black",
				Valid:  true,
			},
			Price: 99900,
		},
	)
	if err != nil {
		t.Fatalf("create variant A: %v", err)
	}

	variantB, err := queries.CreateProductVariant(
		ctx,
		db.CreateProductVariantParams{
			ProductID: product.ID,
			Sku:       "VARIANT-B-" + uuid.NewString(),
			Size: pgtype.Text{
				String: "L",
				Valid:  true,
			},
			Color: pgtype.Text{
				String: "White",
				Valid:  true,
			},
			Price: 109900,
		},
	)
	if err != nil {
		t.Fatalf("create variant B: %v", err)
	}

	t.Cleanup(func() {
		_, _ = pool.Exec(
			ctx,
			`DELETE FROM product_variants WHERE id IN ($1, $2)`,
			variantA.ID,
			variantB.ID,
		)

		_, _ = pool.Exec(
			ctx,
			`DELETE FROM products WHERE id = $1`,
			product.ID,
		)
	})

	service := NewService(pool, queries)

	variants, err := service.ListProductVariants(
		ctx,
		uuid.UUID(product.ID.Bytes),
	)
	if err != nil {
		t.Fatalf("list product variants: %v", err)
	}

	if len(variants) != 2 {
		t.Fatalf(
			"expected 2 variants, got %d",
			len(variants),
		)
	}

	if variants[0].Sku != variantA.Sku {
		t.Fatalf(
			"expected first SKU %s, got %s",
			variantA.Sku,
			variants[0].Sku,
		)
	}

	if variants[1].Sku != variantB.Sku {
		t.Fatalf(
			"expected second SKU %s, got %s",
			variantB.Sku,
			variants[1].Sku,
		)
	}
}

func TestListProductVariantsReturnsEmptyList(t *testing.T) {
	ctx, pool, queries, cleanup := setupProductTest(t)
	defer cleanup()

	product, err := queries.CreateProduct(
		ctx,
		db.CreateProductParams{
			Name: "Product Without Variants",
		},
	)
	if err != nil {
		t.Fatalf("create product: %v", err)
	}

	t.Cleanup(func() {
		_, _ = pool.Exec(
			ctx,
			`DELETE FROM products WHERE id = $1`,
			product.ID,
		)
	})

	service := NewService(pool, queries)

	variants, err := service.ListProductVariants(
		ctx,
		uuid.UUID(product.ID.Bytes),
	)
	if err != nil {
		t.Fatalf("list product variants: %v", err)
	}

	if len(variants) != 0 {
		t.Fatalf(
			"expected 0 variants, got %d",
			len(variants),
		)
	}
}
