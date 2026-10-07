package category

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

func setupCategoryTest(t *testing.T) (
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

func TestCreateCategory(t *testing.T) {
	ctx, pool, queries, cleanup := setupCategoryTest(t)
	defer cleanup()

	service := NewService(pool, queries)

	testID := uuid.NewString()

	category, err := service.CreateCategory(
		ctx,
		"Electronics "+testID,
		"electronics-"+testID,
	)
	if err != nil {
		t.Fatalf("CreateCategory failed: %v", err)
	}

	t.Cleanup(func() {
		_, _ = pool.Exec(
			ctx,
			`DELETE FROM categories WHERE id = $1`,
			category.ID,
		)
	})

	if category.Name != "Electronics "+testID {
		t.Fatalf(
			"expected category name %q, got %q",
			"Electronics "+testID,
			category.Name,
		)
	}

	if category.Slug != "electronics-"+testID {
		t.Fatalf(
			"expected category slug %q, got %q",
			"electronics-"+testID,
			category.Slug,
		)
	}
}

func TestCreateCategoryRejectsEmptyName(t *testing.T) {
	service := NewService(nil, nil)

	_, err := service.CreateCategory(
		context.Background(),
		"   ",
		"electronics",
	)

	if !errors.Is(err, ErrCategoryNameEmpty) {
		t.Fatalf(
			"expected ErrCategoryNameEmpty, got %v",
			err,
		)
	}
}

func TestCreateCategoryRejectsEmptySlug(t *testing.T) {
	service := NewService(nil, nil)

	_, err := service.CreateCategory(
		context.Background(),
		"Electronics",
		"   ",
	)

	if !errors.Is(err, ErrCategorySlugEmpty) {
		t.Fatalf(
			"expected ErrCategorySlugEmpty, got %v",
			err,
		)
	}
}

func TestCreateCategoryRejectsDuplicateSlug(t *testing.T) {
	ctx, pool, queries, cleanup := setupCategoryTest(t)
	defer cleanup()

	service := NewService(pool, queries)

	testID := uuid.NewString()
	slug := "electronics-" + testID

	first, err := service.CreateCategory(
		ctx,
		"Electronics One "+testID,
		slug,
	)
	if err != nil {
		t.Fatalf("create first category: %v", err)
	}

	t.Cleanup(func() {
		_, _ = pool.Exec(
			ctx,
			`DELETE FROM categories WHERE id = $1`,
			first.ID,
		)
	})

	_, err = service.CreateCategory(
		ctx,
		"Electronics Two "+testID,
		slug,
	)
	if err == nil {
		t.Fatal("expected duplicate slug error")
	}
}

func TestGetCategory(t *testing.T) {
	ctx, pool, queries, cleanup := setupCategoryTest(t)
	defer cleanup()

	service := NewService(pool, queries)

	testID := uuid.NewString()

	created, err := service.CreateCategory(
		ctx,
		"Computers "+testID,
		"computers-"+testID,
	)
	if err != nil {
		t.Fatalf("create category: %v", err)
	}

	t.Cleanup(func() {
		_, _ = pool.Exec(
			ctx,
			`DELETE FROM categories WHERE id = $1`,
			created.ID,
		)
	})

	found, err := service.GetCategory(ctx, uuidFromPgtype(created.ID))
	if err != nil {
		t.Fatalf("GetCategory failed: %v", err)
	}

	if found.ID != created.ID {
		t.Fatalf("expected category ID %v, got %v", created.ID, found.ID)
	}

	if found.Name != created.Name {
		t.Fatalf(
			"expected category name %q, got %q",
			created.Name,
			found.Name,
		)
	}
}

func TestGetCategoryNotFound(t *testing.T) {
	ctx, pool, queries, cleanup := setupCategoryTest(t)
	defer cleanup()

	service := NewService(pool, queries)

	_, err := service.GetCategory(
		ctx,
		uuid.New(),
	)

	if !errors.Is(err, ErrCategoryNotFound) {
		t.Fatalf(
			"expected ErrCategoryNotFound, got %v",
			err,
		)
	}
}

func TestGetCategoryBySlug(t *testing.T) {
	ctx, pool, queries, cleanup := setupCategoryTest(t)
	defer cleanup()

	service := NewService(pool, queries)

	testID := uuid.NewString()
	slug := "laptops-" + testID

	created, err := service.CreateCategory(
		ctx,
		"Laptops "+testID,
		slug,
	)
	if err != nil {
		t.Fatalf("create category: %v", err)
	}

	t.Cleanup(func() {
		_, _ = pool.Exec(
			ctx,
			`DELETE FROM categories WHERE id = $1`,
			created.ID,
		)
	})

	found, err := service.GetCategoryBySlug(ctx, slug)
	if err != nil {
		t.Fatalf("GetCategoryBySlug failed: %v", err)
	}

	if found.ID != created.ID {
		t.Fatalf(
			"expected category ID %v, got %v",
			created.ID,
			found.ID,
		)
	}
}

func TestListCategories(t *testing.T) {
	ctx, pool, queries, cleanup := setupCategoryTest(t)
	defer cleanup()

	service := NewService(pool, queries)

	testID := uuid.NewString()

	categoryOne, err := service.CreateCategory(
		ctx,
		"Phones "+testID,
		"phones-"+testID,
	)
	if err != nil {
		t.Fatalf("create category one: %v", err)
	}

	categoryTwo, err := service.CreateCategory(
		ctx,
		"Laptops "+testID,
		"laptops-"+testID,
	)
	if err != nil {
		t.Fatalf("create category two: %v", err)
	}

	t.Cleanup(func() {
		_, _ = pool.Exec(
			ctx,
			`DELETE FROM categories WHERE id IN ($1, $2)`,
			categoryOne.ID,
			categoryTwo.ID,
		)
	})

	categories, err := service.ListCategories(ctx)
	if err != nil {
		t.Fatalf("ListCategories failed: %v", err)
	}

	foundOne := false
	foundTwo := false

	for _, category := range categories {
		if category.ID == categoryOne.ID {
			foundOne = true
		}

		if category.ID == categoryTwo.ID {
			foundTwo = true
		}
	}

	if !foundOne {
		t.Fatal("expected first category to be returned")
	}

	if !foundTwo {
		t.Fatal("expected second category to be returned")
	}
}

func TestProductCategoryRelationship(t *testing.T) {
	ctx, pool, queries, cleanup := setupCategoryTest(t)
	defer cleanup()

	service := NewService(pool, queries)

	testID := uuid.NewString()

	product, err := queries.CreateProduct(
		ctx,
		db.CreateProductParams{
			Name: "Category Test Product " + testID,
		},
	)
	if err != nil {
		t.Fatalf("create product: %v", err)
	}

	category, err := service.CreateCategory(
		ctx,
		"Electronics "+testID,
		"electronics-"+testID,
	)
	if err != nil {
		t.Fatalf("create category: %v", err)
	}

	t.Cleanup(func() {
		_, _ = pool.Exec(
			ctx,
			`DELETE FROM product_categories WHERE product_id = $1`,
			product.ID,
		)

		_, _ = pool.Exec(
			ctx,
			`DELETE FROM products WHERE id = $1`,
			product.ID,
		)

		_, _ = pool.Exec(
			ctx,
			`DELETE FROM categories WHERE id = $1`,
			category.ID,
		)
	})

	productID := uuidFromPgtype(product.ID)
	categoryID := uuidFromPgtype(category.ID)

	err = service.AddProductToCategory(
		ctx,
		productID,
		categoryID,
	)
	if err != nil {
		t.Fatalf("AddProductToCategory failed: %v", err)
	}

	productCategories, err := service.ListProductCategories(
		ctx,
		productID,
	)
	if err != nil {
		t.Fatalf(
			"ListProductCategories failed: %v",
			err,
		)
	}

	foundCategory := false

	for _, item := range productCategories {
		if item.ID == category.ID {
			foundCategory = true
			break
		}
	}

	if !foundCategory {
		t.Fatal("expected category to be assigned to product")
	}

	categoryProducts, err := service.ListCategoryProducts(
		ctx,
		categoryID,
	)
	if err != nil {
		t.Fatalf(
			"ListCategoryProducts failed: %v",
			err,
		)
	}

	foundProduct := false

	for _, item := range categoryProducts {
		if item.ID == product.ID {
			foundProduct = true
			break
		}
	}

	if !foundProduct {
		t.Fatal("expected product to be returned for category")
	}

	err = service.RemoveProductFromCategory(
		ctx,
		productID,
		categoryID,
	)
	if err != nil {
		t.Fatalf(
			"RemoveProductFromCategory failed: %v",
			err,
		)
	}

	productCategories, err = service.ListProductCategories(
		ctx,
		productID,
	)
	if err != nil {
		t.Fatalf(
			"ListProductCategories after removal failed: %v",
			err,
		)
	}

	for _, item := range productCategories {
		if item.ID == category.ID {
			t.Fatal("expected category relationship to be removed")
		}
	}
}

func uuidFromPgtype(value pgtype.UUID) uuid.UUID {
	return value.Bytes
}
