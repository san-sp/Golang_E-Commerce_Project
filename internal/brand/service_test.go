package brand

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

func setupBrandTest(t *testing.T) (
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

func uuidFromPgtype(value pgtype.UUID) uuid.UUID {
	if !value.Valid {
		return uuid.Nil
	}

	return uuid.UUID(value.Bytes)
}

func TestCreateBrand(t *testing.T) {
	ctx, pool, queries, cleanup := setupBrandTest(t)
	defer cleanup()

	service := NewService(pool, queries)

	testID := uuid.NewString()

	brand, err := service.CreateBrand(
		ctx,
		"Nike "+testID,
		"nike-"+testID,
	)
	if err != nil {
		t.Fatalf("CreateBrand failed: %v", err)
	}

	t.Cleanup(func() {
		_, _ = pool.Exec(
			ctx,
			`DELETE FROM brands WHERE id = $1`,
			brand.ID,
		)
	})

	if brand.Name != "Nike "+testID {
		t.Fatalf(
			"expected brand name %q, got %q",
			"Nike "+testID,
			brand.Name,
		)
	}

	if brand.Slug != "nike-"+testID {
		t.Fatalf(
			"expected brand slug %q, got %q",
			"nike-"+testID,
			brand.Slug,
		)
	}
}

func TestCreateBrandRejectsEmptyName(t *testing.T) {
	service := NewService(nil, nil)

	_, err := service.CreateBrand(
		context.Background(),
		"",
		"nike",
	)

	if !errors.Is(err, ErrBrandNameEmpty) {
		t.Fatalf(
			"expected ErrBrandNameEmpty, got %v",
			err,
		)
	}
}

func TestCreateBrandRejectsEmptySlug(t *testing.T) {
	service := NewService(nil, nil)

	_, err := service.CreateBrand(
		context.Background(),
		"Nike",
		"",
	)

	if !errors.Is(err, ErrBrandSlugEmpty) {
		t.Fatalf(
			"expected ErrBrandSlugEmpty, got %v",
			err,
		)
	}
}

func TestCreateBrandRejectsDuplicateSlug(t *testing.T) {
	ctx, pool, queries, cleanup := setupBrandTest(t)
	defer cleanup()

	service := NewService(pool, queries)

	testID := uuid.NewString()
	slug := "nike-" + testID

	first, err := service.CreateBrand(
		ctx,
		"Nike One "+testID,
		slug,
	)
	if err != nil {
		t.Fatalf("create first brand: %v", err)
	}

	t.Cleanup(func() {
		_, _ = pool.Exec(
			ctx,
			`DELETE FROM brands WHERE id = $1`,
			first.ID,
		)
	})

	_, err = service.CreateBrand(
		ctx,
		"Nike Two "+testID,
		slug,
	)
	if err == nil {
		t.Fatal("expected duplicate slug error")
	}
}

func TestGetBrand(t *testing.T) {
	ctx, pool, queries, cleanup := setupBrandTest(t)
	defer cleanup()

	service := NewService(pool, queries)

	testID := uuid.NewString()

	created, err := service.CreateBrand(
		ctx,
		"Adidas "+testID,
		"adidas-"+testID,
	)
	if err != nil {
		t.Fatalf("create brand: %v", err)
	}

	t.Cleanup(func() {
		_, _ = pool.Exec(
			ctx,
			`DELETE FROM brands WHERE id = $1`,
			created.ID,
		)
	})

	found, err := service.GetBrand(
		ctx,
		uuidFromPgtype(created.ID),
	)
	if err != nil {
		t.Fatalf("GetBrand failed: %v", err)
	}

	if found.ID != created.ID {
		t.Fatalf(
			"expected brand ID %v, got %v",
			created.ID,
			found.ID,
		)
	}

	if found.Name != created.Name {
		t.Fatalf(
			"expected brand name %q, got %q",
			created.Name,
			found.Name,
		)
	}
}

func TestGetBrandNotFound(t *testing.T) {
	ctx, pool, queries, cleanup := setupBrandTest(t)
	defer cleanup()

	service := NewService(pool, queries)

	_, err := service.GetBrand(
		ctx,
		uuid.New(),
	)

	if !errors.Is(err, ErrBrandNotFound) {
		t.Fatalf(
			"expected ErrBrandNotFound, got %v",
			err,
		)
	}
}

func TestGetBrandBySlug(t *testing.T) {
	ctx, pool, queries, cleanup := setupBrandTest(t)
	defer cleanup()

	service := NewService(pool, queries)

	testID := uuid.NewString()
	slug := "puma-" + testID

	created, err := service.CreateBrand(
		ctx,
		"Puma "+testID,
		slug,
	)
	if err != nil {
		t.Fatalf("create brand: %v", err)
	}

	t.Cleanup(func() {
		_, _ = pool.Exec(
			ctx,
			`DELETE FROM brands WHERE id = $1`,
			created.ID,
		)
	})

	found, err := service.GetBrandBySlug(ctx, slug)
	if err != nil {
		t.Fatalf("GetBrandBySlug failed: %v", err)
	}

	if found.ID != created.ID {
		t.Fatalf(
			"expected brand ID %v, got %v",
			created.ID,
			found.ID,
		)
	}
}

func TestGetBrandBySlugRejectsEmptySlug(t *testing.T) {
	service := NewService(nil, nil)

	_, err := service.GetBrandBySlug(
		context.Background(),
		"",
	)

	if !errors.Is(err, ErrBrandSlugEmpty) {
		t.Fatalf(
			"expected ErrBrandSlugEmpty, got %v",
			err,
		)
	}
}

func TestGetBrandBySlugNotFound(t *testing.T) {
	ctx, pool, queries, cleanup := setupBrandTest(t)
	defer cleanup()

	service := NewService(pool, queries)

	_, err := service.GetBrandBySlug(
		ctx,
		"does-not-exist-"+uuid.NewString(),
	)

	if !errors.Is(err, ErrBrandNotFound) {
		t.Fatalf(
			"expected ErrBrandNotFound, got %v",
			err,
		)
	}
}

func TestListBrands(t *testing.T) {
	ctx, pool, queries, cleanup := setupBrandTest(t)
	defer cleanup()

	service := NewService(pool, queries)

	testID := uuid.NewString()

	brandOne, err := service.CreateBrand(
		ctx,
		"Apple "+testID,
		"apple-"+testID,
	)
	if err != nil {
		t.Fatalf("create brand one: %v", err)
	}

	brandTwo, err := service.CreateBrand(
		ctx,
		"Samsung "+testID,
		"samsung-"+testID,
	)
	if err != nil {
		t.Fatalf("create brand two: %v", err)
	}

	t.Cleanup(func() {
		_, _ = pool.Exec(
			ctx,
			`DELETE FROM brands WHERE id IN ($1, $2)`,
			brandOne.ID,
			brandTwo.ID,
		)
	})

	brands, err := service.ListBrands(ctx)
	if err != nil {
		t.Fatalf("ListBrands failed: %v", err)
	}

	foundOne := false
	foundTwo := false

	for _, brand := range brands {
		if brand.ID == brandOne.ID {
			foundOne = true
		}

		if brand.ID == brandTwo.ID {
			foundTwo = true
		}
	}

	if !foundOne {
		t.Fatal("expected first brand to be returned")
	}

	if !foundTwo {
		t.Fatal("expected second brand to be returned")
	}
}

func TestAssignProductBrand(t *testing.T) {
	ctx, pool, queries, cleanup := setupBrandTest(t)
	defer cleanup()

	service := NewService(pool, queries)

	testID := uuid.NewString()

	brand, err := service.CreateBrand(
		ctx,
		"Apple "+testID,
		"apple-"+testID,
	)
	if err != nil {
		t.Fatalf("create brand: %v", err)
	}

	productID := uuid.New()

	_, err = pool.Exec(
		ctx,
		`INSERT INTO products (id, name) VALUES ($1, $2)`,
		productID,
		"iPhone "+testID,
	)
	if err != nil {
		t.Fatalf("create product: %v", err)
	}

	t.Cleanup(func() {
		_, _ = pool.Exec(
			ctx,
			`DELETE FROM products WHERE id = $1`,
			productID,
		)

		_, _ = pool.Exec(
			ctx,
			`DELETE FROM brands WHERE id = $1`,
			brand.ID,
		)
	})

	err = service.AssignProductBrand(
		ctx,
		productID,
		uuidFromPgtype(brand.ID),
	)
	if err != nil {
		t.Fatalf("AssignProductBrand failed: %v", err)
	}

	var brandID pgtype.UUID

	err = pool.QueryRow(
		ctx,
		`SELECT brand_id FROM products WHERE id = $1`,
		productID,
	).Scan(&brandID)
	if err != nil {
		t.Fatalf("query product brand: %v", err)
	}

	if !brandID.Valid {
		t.Fatal("expected product brand_id to be set")
	}

	if uuidFromPgtype(brandID) != uuidFromPgtype(brand.ID) {
		t.Fatalf(
			"expected brand ID %v, got %v",
			brand.ID,
			brandID,
		)
	}
}

func TestAssignProductBrandProductNotFound(t *testing.T) {
	ctx, pool, queries, cleanup := setupBrandTest(t)
	defer cleanup()

	service := NewService(pool, queries)

	testID := uuid.NewString()

	brand, err := service.CreateBrand(
		ctx,
		"Apple "+testID,
		"apple-"+testID,
	)
	if err != nil {
		t.Fatalf("create brand: %v", err)
	}

	t.Cleanup(func() {
		_, _ = pool.Exec(
			ctx,
			`DELETE FROM brands WHERE id = $1`,
			brand.ID,
		)
	})

	err = service.AssignProductBrand(
		ctx,
		uuid.New(),
		uuidFromPgtype(brand.ID),
	)
	if !errors.Is(err, ErrProductNotFound) {
		t.Fatalf(
			"expected ErrProductNotFound, got %v",
			err,
		)
	}
}

func TestAssignProductBrandBrandNotFound(t *testing.T) {
	ctx, pool, queries, cleanup := setupBrandTest(t)
	defer cleanup()

	service := NewService(pool, queries)

	productID := uuid.New()

	_, err := pool.Exec(
		ctx,
		`INSERT INTO products (id, name) VALUES ($1, $2)`,
		productID,
		"Test Product "+uuid.NewString(),
	)
	if err != nil {
		t.Fatalf("create product: %v", err)
	}

	t.Cleanup(func() {
		_, _ = pool.Exec(
			ctx,
			`DELETE FROM products WHERE id = $1`,
			productID,
		)
	})

	err = service.AssignProductBrand(
		ctx,
		productID,
		uuid.New(),
	)
	if !errors.Is(err, ErrBrandNotFound) {
		t.Fatalf(
			"expected ErrBrandNotFound, got %v",
			err,
		)
	}
}

func TestClearProductBrand(t *testing.T) {
	ctx, pool, queries, cleanup := setupBrandTest(t)
	defer cleanup()

	service := NewService(pool, queries)

	testID := uuid.NewString()

	brand, err := service.CreateBrand(
		ctx,
		"Samsung "+testID,
		"samsung-"+testID,
	)
	if err != nil {
		t.Fatalf("create brand: %v", err)
	}

	productID := uuid.New()

	_, err = pool.Exec(
		ctx,
		`INSERT INTO products (id, name, brand_id)
		 VALUES ($1, $2, $3)`,
		productID,
		"Galaxy "+testID,
		brand.ID,
	)
	if err != nil {
		t.Fatalf("create product: %v", err)
	}

	t.Cleanup(func() {
		_, _ = pool.Exec(
			ctx,
			`DELETE FROM products WHERE id = $1`,
			productID,
		)

		_, _ = pool.Exec(
			ctx,
			`DELETE FROM brands WHERE id = $1`,
			brand.ID,
		)
	})

	err = service.ClearProductBrand(ctx, productID)
	if err != nil {
		t.Fatalf("ClearProductBrand failed: %v", err)
	}

	var brandID pgtype.UUID

	err = pool.QueryRow(
		ctx,
		`SELECT brand_id FROM products WHERE id = $1`,
		productID,
	).Scan(&brandID)
	if err != nil {
		t.Fatalf("query product brand: %v", err)
	}

	if brandID.Valid {
		t.Fatal("expected product brand_id to be NULL")
	}
}

func TestClearProductBrandProductNotFound(t *testing.T) {
	ctx, pool, queries, cleanup := setupBrandTest(t)
	defer cleanup()

	service := NewService(pool, queries)

	err := service.ClearProductBrand(
		ctx,
		uuid.New(),
	)
	if !errors.Is(err, ErrProductNotFound) {
		t.Fatalf(
			"expected ErrProductNotFound, got %v",
			err,
		)
	}
}

func TestGetProductBrand(t *testing.T) {
	ctx, pool, queries, cleanup := setupBrandTest(t)
	defer cleanup()

	service := NewService(pool, queries)

	testID := uuid.NewString()

	brand, err := service.CreateBrand(
		ctx,
		"Apple "+testID,
		"apple-"+testID,
	)
	if err != nil {
		t.Fatalf("create brand: %v", err)
	}

	productID := uuid.New()

	_, err = pool.Exec(
		ctx,
		`INSERT INTO products (id, name, brand_id)
		 VALUES ($1, $2, $3)`,
		productID,
		"MacBook "+testID,
		brand.ID,
	)
	if err != nil {
		t.Fatalf("create product: %v", err)
	}

	t.Cleanup(func() {
		_, _ = pool.Exec(
			ctx,
			`DELETE FROM products WHERE id = $1`,
			productID,
		)

		_, _ = pool.Exec(
			ctx,
			`DELETE FROM brands WHERE id = $1`,
			brand.ID,
		)
	})

	found, err := service.GetProductBrand(ctx, productID)
	if err != nil {
		t.Fatalf("GetProductBrand failed: %v", err)
	}

	if found.ID != brand.ID {
		t.Fatalf(
			"expected brand ID %v, got %v",
			brand.ID,
			found.ID,
		)
	}
}

func TestGetProductBrandNotFound(t *testing.T) {
	ctx, pool, queries, cleanup := setupBrandTest(t)
	defer cleanup()

	service := NewService(pool, queries)

	_, err := service.GetProductBrand(
		ctx,
		uuid.New(),
	)

	if !errors.Is(err, ErrProductBrandNotFound) {
		t.Fatalf(
			"expected ErrProductBrandNotFound, got %v",
			err,
		)
	}
}

func TestListBrandProducts(t *testing.T) {
	ctx, pool, queries, cleanup := setupBrandTest(t)
	defer cleanup()

	service := NewService(pool, queries)

	testID := uuid.NewString()

	brand, err := service.CreateBrand(
		ctx,
		"Sony "+testID,
		"sony-"+testID,
	)
	if err != nil {
		t.Fatalf("create brand: %v", err)
	}

	productOneID := uuid.New()
	productTwoID := uuid.New()

	_, err = pool.Exec(
		ctx,
		`INSERT INTO products (id, name, brand_id)
		 VALUES
		 ($1, $2, $3),
		 ($4, $5, $3)`,
		productOneID,
		"PlayStation "+testID,
		brand.ID,
		productTwoID,
		"Headphones "+testID,
	)
	if err != nil {
		t.Fatalf("create products: %v", err)
	}

	t.Cleanup(func() {
		_, _ = pool.Exec(
			ctx,
			`DELETE FROM products WHERE id IN ($1, $2)`,
			productOneID,
			productTwoID,
		)

		_, _ = pool.Exec(
			ctx,
			`DELETE FROM brands WHERE id = $1`,
			brand.ID,
		)
	})

	products, err := service.ListBrandProducts(
		ctx,
		uuidFromPgtype(brand.ID),
	)
	if err != nil {
		t.Fatalf("ListBrandProducts failed: %v", err)
	}

	foundOne := false
	foundTwo := false

	for _, product := range products {
		if product.ID.Bytes == productOneID {
			foundOne = true
		}

		if product.ID.Bytes == productTwoID {
			foundTwo = true
		}
	}

	if !foundOne {
		t.Fatal("expected first product to be returned")
	}

	if !foundTwo {
		t.Fatal("expected second product to be returned")
	}
}

func TestListBrandProductsBrandNotFound(t *testing.T) {
	ctx, pool, queries, cleanup := setupBrandTest(t)
	defer cleanup()

	service := NewService(pool, queries)

	_, err := service.ListBrandProducts(
		ctx,
		uuid.New(),
	)

	if !errors.Is(err, ErrBrandNotFound) {
		t.Fatalf(
			"expected ErrBrandNotFound, got %v",
			err,
		)
	}
}
