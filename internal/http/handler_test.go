package http

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/joho/godotenv"

	"github.com/san-sp/Golang_E-Commerce_Project/internal/checkout"
	"github.com/san-sp/Golang_E-Commerce_Project/internal/database"
	"github.com/san-sp/Golang_E-Commerce_Project/internal/database/db"
	"github.com/san-sp/Golang_E-Commerce_Project/internal/order"
	"github.com/san-sp/Golang_E-Commerce_Project/internal/payment"
	"github.com/san-sp/Golang_E-Commerce_Project/internal/product"
	"github.com/san-sp/Golang_E-Commerce_Project/internal/reservation"
)

func setupCheckoutHandler(t *testing.T) (
	*Handler,
	*db.Queries,
	*payment.MockProvider,
	*pgxpool.Pool,
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

	orderService := order.NewService(
		pool,
		queries,
	)

	reservationService := reservation.NewService(
		pool,
		queries,
		15*time.Minute,
	)

	provider := payment.NewMockProvider()

	paymentService := payment.NewService(
		pool,
		queries,
		provider,
	)

	checkoutService := checkout.NewService(
		pool,
		queries,
		orderService,
		reservationService,
		paymentService,
	)

	handler := NewHandler(
		paymentService,
		nil,
		reservationService,
		nil,
		checkoutService,
		orderService,
		nil,
	)

	return handler, queries, provider, pool
}

func checkoutTestVariantID() pgtype.UUID {
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

// Product
func TestGetProduct(t *testing.T) {
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
	defer pool.Close()

	queries := db.New(pool)

	createdProduct, err := queries.CreateProduct(
		ctx,
		db.CreateProductParams{
			Name: "HTTP Test Product",
		},
	)
	if err != nil {
		t.Fatalf("create product: %v", err)
	}

	t.Cleanup(func() {
		_, _ = pool.Exec(
			ctx,
			`DELETE FROM products WHERE id = $1`,
			createdProduct.ID,
		)
	})

	productService := product.NewService(pool, queries)

	handler := NewHandler(
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		productService,
	)

	router := gin.New()

	router.GET(
		"/api/v1/products/:id",
		handler.GetProduct,
	)

	req := httptest.NewRequest(
		http.MethodGet,
		"/api/v1/products/"+uuid.UUID(createdProduct.ID.Bytes).String(),
		nil,
	)

	recorder := httptest.NewRecorder()

	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf(
			"expected status 200, got %d: %s",
			recorder.Code,
			recorder.Body.String(),
		)
	}
}

func TestGetProductInvalidID(t *testing.T) {
	handler := NewHandler(
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
	)

	router := gin.New()

	router.GET(
		"/api/v1/products/:id",
		handler.GetProduct,
	)

	req := httptest.NewRequest(
		http.MethodGet,
		"/api/v1/products/not-a-uuid",
		nil,
	)

	recorder := httptest.NewRecorder()

	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf(
			"expected status 400, got %d",
			recorder.Code,
		)
	}
}

func TestGetProductNotFound(t *testing.T) {
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
	defer pool.Close()

	queries := db.New(pool)

	productService := product.NewService(pool, queries)

	handler := NewHandler(
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		productService,
	)

	router := gin.New()

	router.GET(
		"/api/v1/products/:id",
		handler.GetProduct,
	)

	req := httptest.NewRequest(
		http.MethodGet,
		"/api/v1/products/"+uuid.NewString(),
		nil,
	)

	recorder := httptest.NewRecorder()

	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusNotFound {
		t.Fatalf(
			"expected status 404, got %d: %s",
			recorder.Code,
			recorder.Body.String(),
		)
	}
}

func TestListProducts(t *testing.T) {
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
	defer pool.Close()

	queries := db.New(pool)

	productA, err := queries.CreateProduct(
		ctx,
		db.CreateProductParams{
			Name: "HTTP List Product A",
		},
	)
	if err != nil {
		t.Fatalf("create product A: %v", err)
	}

	productB, err := queries.CreateProduct(
		ctx,
		db.CreateProductParams{
			Name: "HTTP List Product B",
		},
	)
	if err != nil {
		t.Fatalf("create product B: %v", err)
	}

	t.Cleanup(func() {
		_, _ = pool.Exec(
			ctx,
			`DELETE FROM products WHERE id IN ($1, $2)`,
			productA.ID,
			productB.ID,
		)
	})

	productService := product.NewService(pool, queries)

	handler := NewHandler(
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		productService,
	)

	router := gin.New()

	router.GET(
		"/api/v1/products",
		handler.ListProducts,
	)

	req := httptest.NewRequest(
		http.MethodGet,
		"/api/v1/products?page=1&limit=20",
		nil,
	)

	recorder := httptest.NewRecorder()

	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf(
			"expected status 200, got %d: %s",
			recorder.Code,
			recorder.Body.String(),
		)
	}

	if recorder.Body.Len() == 0 {
		t.Fatal("expected response body")
	}
}

func TestListProductsInvalidPage(t *testing.T) {
	handler := NewHandler(
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
	)

	router := gin.New()

	router.GET(
		"/api/v1/products",
		handler.ListProducts,
	)

	req := httptest.NewRequest(
		http.MethodGet,
		"/api/v1/products?page=abc",
		nil,
	)

	recorder := httptest.NewRecorder()

	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf(
			"expected status 400, got %d",
			recorder.Code,
		)
	}
}

func TestListProductsInvalidLimit(t *testing.T) {
	handler := NewHandler(
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
	)

	router := gin.New()

	router.GET(
		"/api/v1/products",
		handler.ListProducts,
	)

	req := httptest.NewRequest(
		http.MethodGet,
		"/api/v1/products?limit=101",
		nil,
	)

	recorder := httptest.NewRecorder()

	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf(
			"expected status 400, got %d",
			recorder.Code,
		)
	}
}

func TestListProductsSortCreatedAsc(t *testing.T) {
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
	defer pool.Close()

	queries := db.New(pool)

	productA, err := queries.CreateProduct(
		ctx,
		db.CreateProductParams{
			Name: "HTTP Sort Product A",
		},
	)
	if err != nil {
		t.Fatalf("create product A: %v", err)
	}

	productB, err := queries.CreateProduct(
		ctx,
		db.CreateProductParams{
			Name: "HTTP Sort Product B",
		},
	)
	if err != nil {
		t.Fatalf("create product B: %v", err)
	}

	t.Cleanup(func() {
		_, _ = pool.Exec(
			ctx,
			`DELETE FROM products WHERE id IN ($1, $2)`,
			productA.ID,
			productB.ID,
		)
	})

	_, err = pool.Exec(
		ctx,
		`UPDATE products
		 SET created_at = CASE
		     WHEN id = $1 THEN TIMESTAMPTZ '2020-01-01 00:00:00+00'
		     WHEN id = $2 THEN TIMESTAMPTZ '2020-01-02 00:00:00+00'
		 END
		 WHERE id IN ($1, $2)`,
		productA.ID,
		productB.ID,
	)
	if err != nil {
		t.Fatalf("set test timestamps: %v", err)
	}

	productService := product.NewService(pool, queries)

	handler := NewHandler(
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		productService,
	)

	router := gin.New()

	router.GET(
		"/api/v1/products",
		handler.ListProducts,
	)

	req := httptest.NewRequest(
		http.MethodGet,
		"/api/v1/products?page=1&limit=100&sort=created_asc",
		nil,
	)

	recorder := httptest.NewRecorder()

	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf(
			"expected status 200, got %d: %s",
			recorder.Code,
			recorder.Body.String(),
		)
	}

	var products []db.Product

	if err := json.Unmarshal(recorder.Body.Bytes(), &products); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	firstIndex := -1
	secondIndex := -1

	for i, p := range products {
		if p.ID == productA.ID {
			firstIndex = i
		}

		if p.ID == productB.ID {
			secondIndex = i
		}
	}

	if firstIndex == -1 {
		t.Fatal("product A was not returned")
	}

	if secondIndex == -1 {
		t.Fatal("product B was not returned")
	}

	if firstIndex >= secondIndex {
		t.Fatalf(
			"expected product A at index %d before product B at index %d",
			firstIndex,
			secondIndex,
		)
	}
}

func TestListProductsInvalidSort(t *testing.T) {
	productService := product.NewService(nil, nil)

	handler := NewHandler(
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		productService,
	)

	router := gin.New()

	router.GET(
		"/api/v1/products",
		handler.ListProducts,
	)

	req := httptest.NewRequest(
		http.MethodGet,
		"/api/v1/products?sort=drop_database",
		nil,
	)

	recorder := httptest.NewRecorder()

	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf(
			"expected status 400, got %d: %s",
			recorder.Code,
			recorder.Body.String(),
		)
	}
}

func TestListProductVariants(t *testing.T) {
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
	defer pool.Close()

	queries := db.New(pool)

	productRecord, err := queries.CreateProduct(
		ctx,
		db.CreateProductParams{
			Name: "HTTP Variant Product",
		},
	)
	if err != nil {
		t.Fatalf("create product: %v", err)
	}

	variantRecord, err := queries.CreateProductVariant(
		ctx,
		db.CreateProductVariantParams{
			ProductID: productRecord.ID,
			Sku:       "HTTP-VARIANT-" + uuid.NewString(),
			Price:     129900,
		},
	)
	if err != nil {
		t.Fatalf("create variant: %v", err)
	}

	t.Cleanup(func() {
		_, _ = pool.Exec(
			ctx,
			`DELETE FROM product_variants WHERE id = $1`,
			variantRecord.ID,
		)

		_, _ = pool.Exec(
			ctx,
			`DELETE FROM products WHERE id = $1`,
			productRecord.ID,
		)
	})

	productService := product.NewService(pool, queries)

	handler := NewHandler(
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		productService,
	)

	router := gin.New()

	router.GET(
		"/api/v1/products/:id/variants",
		handler.ListProductVariants,
	)

	req := httptest.NewRequest(
		http.MethodGet,
		"/api/v1/products/"+uuid.UUID(productRecord.ID.Bytes).String()+"/variants",
		nil,
	)

	recorder := httptest.NewRecorder()

	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf(
			"expected status 200, got %d: %s",
			recorder.Code,
			recorder.Body.String(),
		)
	}

	if recorder.Body.Len() == 0 {
		t.Fatal("expected response body")
	}
}

func TestListProductVariantsInvalidID(t *testing.T) {
	handler := NewHandler(
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
	)

	router := gin.New()

	router.GET(
		"/api/v1/products/:id/variants",
		handler.ListProductVariants,
	)

	req := httptest.NewRequest(
		http.MethodGet,
		"/api/v1/products/not-a-uuid/variants",
		nil,
	)

	recorder := httptest.NewRecorder()

	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf(
			"expected status 400, got %d",
			recorder.Code,
		)
	}
}

func TestListProductVariantsEmpty(t *testing.T) {
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
	defer pool.Close()

	queries := db.New(pool)

	productRecord, err := queries.CreateProduct(
		ctx,
		db.CreateProductParams{
			Name: "HTTP Empty Variant Product",
		},
	)
	if err != nil {
		t.Fatalf("create product: %v", err)
	}

	t.Cleanup(func() {
		_, _ = pool.Exec(
			ctx,
			`DELETE FROM products WHERE id = $1`,
			productRecord.ID,
		)
	})

	productService := product.NewService(pool, queries)

	handler := NewHandler(
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		productService,
	)

	router := gin.New()

	router.GET(
		"/api/v1/products/:id/variants",
		handler.ListProductVariants,
	)

	req := httptest.NewRequest(
		http.MethodGet,
		"/api/v1/products/"+uuid.UUID(productRecord.ID.Bytes).String()+"/variants",
		nil,
	)

	recorder := httptest.NewRecorder()

	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf(
			"expected status 200, got %d: %s",
			recorder.Code,
			recorder.Body.String(),
		)
	}

	if recorder.Body.String() != "[]" {
		t.Fatalf(
			"expected empty JSON array, got %s",
			recorder.Body.String(),
		)
	}
}

// Checkout
func TestCheckoutHandlerSuccess(t *testing.T) {
	handler, queries, _, pool := setupCheckoutHandler(t)

	ctx := context.Background()

	cart, err := queries.CreateCart(ctx)
	if err != nil {
		t.Fatalf("create cart: %v", err)
	}

	t.Cleanup(func() {
		_, _ = pool.Exec(
			ctx,
			`DELETE FROM carts WHERE id = $1`,
			cart.ID,
		)
	})

	_, err = queries.AddCartItem(
		ctx,
		db.AddCartItemParams{
			CartID:    cart.ID,
			VariantID: checkoutTestVariantID(),
			Quantity:  1,
		},
	)
	if err != nil {
		t.Fatalf("add cart item: %v", err)
	}

	gin.SetMode(gin.TestMode)

	router := gin.New()

	router.POST(
		"/api/v1/checkout",
		handler.Checkout,
	)

	requestBody := map[string]any{
		"cart_id":  cart.ID,
		"currency": "INR",
		"provider": "mock",
	}

	body, err := json.Marshal(requestBody)
	if err != nil {
		t.Fatalf("marshal request body: %v", err)
	}

	request := httptest.NewRequest(
		http.MethodPost,
		"/api/v1/checkout",
		bytes.NewReader(body),
	)

	request.Header.Set(
		"Content-Type",
		"application/json",
	)

	recorder := httptest.NewRecorder()

	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusCreated {
		t.Fatalf(
			"expected status 201, got %d: %s",
			recorder.Code,
			recorder.Body.String(),
		)
	}

	var response struct {
		Order struct {
			ID          string `json:"id"`
			Status      string `json:"status"`
			TotalAmount int64  `json:"total_amount"`
			Currency    string `json:"currency"`
		} `json:"order"`

		Payment struct {
			ID     string `json:"id"`
			Status string `json:"status"`
			Amount int64  `json:"amount"`
		} `json:"payment"`
	}

	err = json.Unmarshal(
		recorder.Body.Bytes(),
		&response,
	)
	if err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if response.Order.ID == "" {
		t.Fatal("expected order ID")
	}

	if response.Order.Status != "PENDING" {
		t.Fatalf(
			"expected order status PENDING, got %s",
			response.Order.Status,
		)
	}

	if response.Order.TotalAmount != 899900 {
		t.Fatalf(
			"expected total amount 899900, got %d",
			response.Order.TotalAmount,
		)
	}

	if response.Order.Currency != "INR" {
		t.Fatalf(
			"expected currency INR, got %s",
			response.Order.Currency,
		)
	}

	if response.Payment.ID == "" {
		t.Fatal("expected payment ID")
	}

	if response.Payment.Status != "PENDING" {
		t.Fatalf(
			"expected payment status PENDING, got %s",
			response.Payment.Status,
		)
	}

	if response.Payment.Amount != 899900 {
		t.Fatalf(
			"expected payment amount 899900, got %d",
			response.Payment.Amount,
		)
	}

	t.Cleanup(func() {
		_, _ = pool.Exec(
			ctx,
			`DELETE FROM payments WHERE order_id = $1`,
			response.Order.ID,
		)

		_, _ = pool.Exec(
			ctx,
			`DELETE FROM reservations WHERE order_id = $1`,
			response.Order.ID,
		)

		_, _ = pool.Exec(
			ctx,
			`DELETE FROM orders WHERE id = $1`,
			response.Order.ID,
		)
	})
}

func TestCheckoutHandlerInvalidCartID(t *testing.T) {
	handler, _, _, _ := setupCheckoutHandler(t)

	gin.SetMode(gin.TestMode)

	router := gin.New()

	router.POST(
		"/api/v1/checkout",
		handler.Checkout,
	)

	requestBody := map[string]any{
		"cart_id":  "not-a-uuid",
		"currency": "INR",
		"provider": "mock",
	}

	body, err := json.Marshal(requestBody)
	if err != nil {
		t.Fatalf("marshal request body: %v", err)
	}

	request := httptest.NewRequest(
		http.MethodPost,
		"/api/v1/checkout",
		bytes.NewReader(body),
	)

	request.Header.Set(
		"Content-Type",
		"application/json",
	)

	recorder := httptest.NewRecorder()

	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf(
			"expected status 400, got %d: %s",
			recorder.Code,
			recorder.Body.String(),
		)
	}

	var response struct {
		Error ErrorResponse `json:"error"`
	}

	err = json.Unmarshal(
		recorder.Body.Bytes(),
		&response,
	)
	if err != nil {
		t.Fatalf("decode error response: %v", err)
	}

	if response.Error.Code != "INVALID_REQUEST" {
		t.Fatalf(
			"expected error code INVALID_REQUEST, got %s",
			response.Error.Code,
		)
	}
}

func TestCheckoutHandlerEmptyCart(t *testing.T) {
	handler, queries, _, pool := setupCheckoutHandler(t)

	ctx := context.Background()

	cart, err := queries.CreateCart(ctx)
	if err != nil {
		t.Fatalf("create cart: %v", err)
	}

	t.Cleanup(func() {
		_, _ = pool.Exec(
			ctx,
			`DELETE FROM carts WHERE id = $1`,
			cart.ID,
		)
	})

	gin.SetMode(gin.TestMode)

	router := gin.New()

	router.POST(
		"/api/v1/checkout",
		handler.Checkout,
	)

	requestBody := map[string]any{
		"cart_id":  cart.ID,
		"currency": "INR",
		"provider": "mock",
	}

	body, err := json.Marshal(requestBody)
	if err != nil {
		t.Fatalf("marshal request body: %v", err)
	}

	request := httptest.NewRequest(
		http.MethodPost,
		"/api/v1/checkout",
		bytes.NewReader(body),
	)

	request.Header.Set(
		"Content-Type",
		"application/json",
	)

	recorder := httptest.NewRecorder()

	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusConflict {
		t.Fatalf(
			"expected status 409, got %d: %s",
			recorder.Code,
			recorder.Body.String(),
		)
	}

	var response struct {
		Error ErrorResponse `json:"error"`
	}

	err = json.Unmarshal(
		recorder.Body.Bytes(),
		&response,
	)
	if err != nil {
		t.Fatalf("decode error response: %v", err)
	}

	if response.Error.Code != "CART_EMPTY" {
		t.Fatalf(
			"expected error code CART_EMPTY, got %s",
			response.Error.Code,
		)
	}

	if response.Error.Message != "cart is empty" {
		t.Fatalf(
			"expected error message %q, got %q",
			"cart is empty",
			response.Error.Message,
		)
	}
}

func TestCheckoutHandlerProviderFailed(t *testing.T) {
	handler, queries, provider, pool := setupCheckoutHandler(t)

	ctx := context.Background()

	provider.SetFailure(true)

	cart, err := queries.CreateCart(ctx)
	if err != nil {
		t.Fatalf("create cart: %v", err)
	}

	t.Cleanup(func() {
		_, _ = pool.Exec(
			ctx,
			`DELETE FROM carts WHERE id = $1`,
			cart.ID,
		)
	})

	_, err = queries.AddCartItem(
		ctx,
		db.AddCartItemParams{
			CartID:    cart.ID,
			VariantID: checkoutTestVariantID(),
			Quantity:  1,
		},
	)
	if err != nil {
		t.Fatalf("add cart item: %v", err)
	}

	gin.SetMode(gin.TestMode)

	router := gin.New()

	router.POST(
		"/api/v1/checkout",
		handler.Checkout,
	)

	requestBody := map[string]any{
		"cart_id":  cart.ID,
		"currency": "INR",
		"provider": "mock",
	}

	body, err := json.Marshal(requestBody)
	if err != nil {
		t.Fatalf("marshal request body: %v", err)
	}

	request := httptest.NewRequest(
		http.MethodPost,
		"/api/v1/checkout",
		bytes.NewReader(body),
	)

	request.Header.Set(
		"Content-Type",
		"application/json",
	)

	recorder := httptest.NewRecorder()

	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusBadGateway {
		t.Fatalf(
			"expected status 502, got %d: %s",
			recorder.Code,
			recorder.Body.String(),
		)
	}

	var response struct {
		Error ErrorResponse `json:"error"`
	}

	if err := json.Unmarshal(
		recorder.Body.Bytes(),
		&response,
	); err != nil {
		t.Fatalf("decode error response: %v", err)
	}

	if response.Error.Code != "PAYMENT_FAILED" {
		t.Fatalf(
			"expected error code PAYMENT_FAILED, got %s",
			response.Error.Code,
		)
	}

	if response.Error.Message != "payment provider failed" {
		t.Fatalf(
			"expected error message %q, got %q",
			"payment provider failed",
			response.Error.Message,
		)
	}
}

func TestCheckoutHandlerProviderUnknown(t *testing.T) {
	handler, queries, provider, pool := setupCheckoutHandler(t)

	ctx := context.Background()

	provider.SetUnknown()

	cart, err := queries.CreateCart(ctx)
	if err != nil {
		t.Fatalf("create cart: %v", err)
	}

	t.Cleanup(func() {
		_, _ = pool.Exec(
			ctx,
			`DELETE FROM carts WHERE id = $1`,
			cart.ID,
		)
	})

	_, err = queries.AddCartItem(
		ctx,
		db.AddCartItemParams{
			CartID:    cart.ID,
			VariantID: checkoutTestVariantID(),
			Quantity:  1,
		},
	)
	if err != nil {
		t.Fatalf("add cart item: %v", err)
	}

	gin.SetMode(gin.TestMode)

	router := gin.New()

	router.POST(
		"/api/v1/checkout",
		handler.Checkout,
	)

	requestBody := map[string]any{
		"cart_id":  cart.ID,
		"currency": "INR",
		"provider": "mock",
	}

	body, err := json.Marshal(requestBody)
	if err != nil {
		t.Fatalf("marshal request body: %v", err)
	}

	request := httptest.NewRequest(
		http.MethodPost,
		"/api/v1/checkout",
		bytes.NewReader(body),
	)

	request.Header.Set(
		"Content-Type",
		"application/json",
	)

	recorder := httptest.NewRecorder()

	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusAccepted {
		t.Fatalf(
			"expected status 202, got %d: %s",
			recorder.Code,
			recorder.Body.String(),
		)
	}

	var response struct {
		Error ErrorResponse `json:"error"`
	}

	if err := json.Unmarshal(
		recorder.Body.Bytes(),
		&response,
	); err != nil {
		t.Fatalf("decode error response: %v", err)
	}

	if response.Error.Code != "PAYMENT_OUTCOME_UNKNOWN" {
		t.Fatalf(
			"expected error code PAYMENT_OUTCOME_UNKNOWN, got %s",
			response.Error.Code,
		)
	}

	if response.Error.Message != "payment outcome is unknown; await payment confirmation" {
		t.Fatalf(
			"expected error message %q, got %q",
			"payment outcome is unknown; await payment confirmation",
			response.Error.Message,
		)
	}
}

// Reservation
func TestCreateReservationHandler(t *testing.T) {
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

	reservationService := reservation.NewService(
		pool,
		queries,
		15*time.Minute,
	)

	handler := NewHandler(
		nil,
		nil,
		reservationService,
		nil,
		nil,
		nil,
		nil,
	)

	gin.SetMode(gin.TestMode)

	router := gin.New()

	router.POST(
		"/api/v1/reservations",
		handler.CreateReservation,
	)

	requestBody := map[string]any{
		"variant_id": "c5174f98-5ba2-453e-92fb-266e818fbd92",
		"quantity":   1,
	}

	body, err := json.Marshal(requestBody)
	if err != nil {
		t.Fatalf("marshal request body: %v", err)
	}

	request := httptest.NewRequest(
		http.MethodPost,
		"/api/v1/reservations",
		bytes.NewReader(body),
	)

	request.Header.Set("Content-Type", "application/json")

	recorder := httptest.NewRecorder()

	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusCreated {
		t.Fatalf(
			"expected status 201, got %d: %s",
			recorder.Code,
			recorder.Body.String(),
		)
	}

	var response struct {
		ID        string `json:"id"`
		VariantID string `json:"variant_id"`
		Quantity  int64  `json:"quantity"`
		Status    string `json:"status"`
	}

	err = json.Unmarshal(
		recorder.Body.Bytes(),
		&response,
	)
	if err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if response.ID == "" {
		t.Fatal("expected reservation id")
	}

	if response.VariantID != requestBody["variant_id"] {
		t.Fatalf(
			"expected variant_id %v, got %s",
			requestBody["variant_id"],
			response.VariantID,
		)
	}

	if response.Quantity != 1 {
		t.Fatalf(
			"expected quantity 1, got %d",
			response.Quantity,
		)
	}

	if response.Status != "ACTIVE" {
		t.Fatalf(
			"expected status ACTIVE, got %s",
			response.Status,
		)
	}

	t.Cleanup(func() {
		_, err := pool.Exec(
			ctx,
			"DELETE FROM reservations WHERE id = $1",
			response.ID,
		)
		if err != nil {
			t.Logf("cleanup reservation failed: %v", err)
		}
	})
}

func TestCreateReservationHandlerInsufficientStock(t *testing.T) {
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

	reservationService := reservation.NewService(
		pool,
		queries,
		15*time.Minute,
	)

	handler := NewHandler(
		nil,
		nil,
		reservationService,
		nil,
		nil,
		nil,
		nil,
	)

	gin.SetMode(gin.TestMode)

	router := gin.New()

	router.POST(
		"/api/v1/reservations",
		handler.CreateReservation,
	)

	requestBody := map[string]any{
		"variant_id": "c5174f98-5ba2-453e-92fb-266e818fbd92",
		"quantity":   999999,
	}

	body, err := json.Marshal(requestBody)
	if err != nil {
		t.Fatalf("marshal request body: %v", err)
	}

	request := httptest.NewRequest(
		http.MethodPost,
		"/api/v1/reservations",
		bytes.NewReader(body),
	)

	request.Header.Set("Content-Type", "application/json")

	recorder := httptest.NewRecorder()

	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusConflict {
		t.Fatalf(
			"expected status 409, got %d: %s",
			recorder.Code,
			recorder.Body.String(),
		)
	}

	var response struct {
		Error ErrorResponse `json:"error"`
	}

	err = json.Unmarshal(
		recorder.Body.Bytes(),
		&response,
	)
	if err != nil {
		t.Fatalf("decode error response: %v", err)
	}

	if response.Error.Code != "INSUFFICIENT_STOCK" {
		t.Fatalf(
			"expected error code INSUFFICIENT_STOCK, got %s",
			response.Error.Code,
		)
	}

	if response.Error.Message != "insufficient stock" {
		t.Fatalf(
			"expected error message %q, got %q",
			"insufficient stock",
			response.Error.Message,
		)
	}
}

// Order
func TestGetOrderHandler(t *testing.T) {
	handler, queries, _, pool := setupCheckoutHandler(t)

	ctx := context.Background()

	orderRecord, err := queries.CreateOrder(
		ctx,
		db.CreateOrderParams{
			TotalAmount: 1799800,
			Currency:    "INR",
		},
	)
	if err != nil {
		t.Fatalf("create order: %v", err)
	}

	_, err = queries.CreateOrderItem(
		ctx,
		db.CreateOrderItemParams{
			OrderID:   orderRecord.ID,
			VariantID: checkoutTestVariantID(),
			Quantity:  2,
			UnitPrice: 899900,
		},
	)
	if err != nil {
		t.Fatalf("create order item: %v", err)
	}

	t.Cleanup(func() {
		_, _ = pool.Exec(
			ctx,
			`DELETE FROM orders WHERE id = $1`,
			orderRecord.ID,
		)
	})

	gin.SetMode(gin.TestMode)

	router := gin.New()
	router.GET("/api/v1/orders/:id", handler.GetOrder)

	request := httptest.NewRequest(
		http.MethodGet,
		"/api/v1/orders/"+orderRecord.ID.String(),
		nil,
	)

	recorder := httptest.NewRecorder()

	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf(
			"expected status 200, got %d: %s",
			recorder.Code,
			recorder.Body.String(),
		)
	}

	var response struct {
		ID          string `json:"id"`
		Status      string `json:"status"`
		TotalAmount int64  `json:"total_amount"`
		Currency    string `json:"currency"`
		Items       []struct {
			VariantID string `json:"variant_id"`
			Quantity  int64  `json:"quantity"`
			UnitPrice int64  `json:"unit_price"`
		} `json:"items"`
	}

	if err := json.Unmarshal(
		recorder.Body.Bytes(),
		&response,
	); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if response.ID != orderRecord.ID.String() {
		t.Fatalf(
			"expected order ID %s, got %s",
			orderRecord.ID,
			response.ID,
		)
	}

	if response.Status != "PENDING" {
		t.Fatalf(
			"expected status PENDING, got %s",
			response.Status,
		)
	}

	if response.TotalAmount != 1799800 {
		t.Fatalf(
			"expected total amount 1799800, got %d",
			response.TotalAmount,
		)
	}

	if response.Currency != "INR" {
		t.Fatalf(
			"expected currency INR, got %s",
			response.Currency,
		)
	}

	if len(response.Items) != 1 {
		t.Fatalf(
			"expected 1 order item, got %d",
			len(response.Items),
		)
	}

	if response.Items[0].Quantity != 2 {
		t.Fatalf(
			"expected quantity 2, got %d",
			response.Items[0].Quantity,
		)
	}

	if response.Items[0].UnitPrice != 899900 {
		t.Fatalf(
			"expected unit price 899900, got %d",
			response.Items[0].UnitPrice,
		)
	}
}

func TestGetOrderHandlerNotFound(t *testing.T) {
	handler, _, _, _ := setupCheckoutHandler(t)

	gin.SetMode(gin.TestMode)

	router := gin.New()
	router.GET("/api/v1/orders/:id", handler.GetOrder)

	orderID := uuid.New()

	request := httptest.NewRequest(
		http.MethodGet,
		"/api/v1/orders/"+orderID.String(),
		nil,
	)

	recorder := httptest.NewRecorder()

	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusNotFound {
		t.Fatalf(
			"expected status 404, got %d: %s",
			recorder.Code,
			recorder.Body.String(),
		)
	}

	var response struct {
		Error ErrorResponse `json:"error"`
	}

	if err := json.Unmarshal(
		recorder.Body.Bytes(),
		&response,
	); err != nil {
		t.Fatalf("decode error response: %v", err)
	}

	if response.Error.Code != "ORDER_NOT_FOUND" {
		t.Fatalf(
			"expected error code ORDER_NOT_FOUND, got %s",
			response.Error.Code,
		)
	}

	if response.Error.Message != "order not found" {
		t.Fatalf(
			"expected error message %q, got %q",
			"order not found",
			response.Error.Message,
		)
	}
}

func TestCancelOrderHandler(t *testing.T) {
	handler, queries, _, pool := setupCheckoutHandler(t)

	ctx := context.Background()

	// Create cart.
	cart, err := queries.CreateCart(ctx)
	if err != nil {
		t.Fatalf("create cart: %v", err)
	}

	t.Cleanup(func() {
		_, _ = pool.Exec(
			ctx,
			`DELETE FROM carts WHERE id = $1`,
			cart.ID,
		)
	})

	// Add item to cart.
	_, err = queries.AddCartItem(
		ctx,
		db.AddCartItemParams{
			CartID:    cart.ID,
			VariantID: checkoutTestVariantID(),
			Quantity:  1,
		},
	)
	if err != nil {
		t.Fatalf("add cart item: %v", err)
	}

	// Checkout.
	gin.SetMode(gin.TestMode)

	checkoutRouter := gin.New()

	checkoutRouter.POST(
		"/api/v1/checkout",
		handler.Checkout,
	)

	requestBody := map[string]any{
		"cart_id":  cart.ID,
		"currency": "INR",
		"provider": "mock",
	}

	body, err := json.Marshal(requestBody)
	if err != nil {
		t.Fatalf("marshal request body: %v", err)
	}

	checkoutRequest := httptest.NewRequest(
		http.MethodPost,
		"/api/v1/checkout",
		bytes.NewReader(body),
	)

	checkoutRequest.Header.Set(
		"Content-Type",
		"application/json",
	)

	checkoutRecorder := httptest.NewRecorder()

	checkoutRouter.ServeHTTP(
		checkoutRecorder,
		checkoutRequest,
	)

	if checkoutRecorder.Code != http.StatusCreated {
		t.Fatalf(
			"expected checkout status 201, got %d: %s",
			checkoutRecorder.Code,
			checkoutRecorder.Body.String(),
		)
	}

	var checkoutResponse struct {
		Order struct {
			ID string `json:"id"`
		} `json:"order"`
	}

	err = json.Unmarshal(
		checkoutRecorder.Body.Bytes(),
		&checkoutResponse,
	)
	if err != nil {
		t.Fatalf("decode checkout response: %v", err)
	}

	if checkoutResponse.Order.ID == "" {
		t.Fatal("expected order ID")
	}

	orderID := checkoutResponse.Order.ID

	// Cancel order.
	cancelRouter := gin.New()

	cancelRouter.POST(
		"/api/v1/orders/:id/cancel",
		handler.CancelOrder,
	)

	cancelRequest := httptest.NewRequest(
		http.MethodPost,
		"/api/v1/orders/"+orderID+"/cancel",
		nil,
	)

	cancelRecorder := httptest.NewRecorder()

	cancelRouter.ServeHTTP(
		cancelRecorder,
		cancelRequest,
	)

	if cancelRecorder.Code != http.StatusOK {
		t.Fatalf(
			"expected cancellation status 200, got %d: %s",
			cancelRecorder.Code,
			cancelRecorder.Body.String(),
		)
	}

	var cancelResponse struct {
		Order struct {
			ID     string `json:"id"`
			Status string `json:"status"`
		} `json:"order"`

		Reservations []struct {
			Status string `json:"status"`
		} `json:"reservations"`
	}

	err = json.Unmarshal(
		cancelRecorder.Body.Bytes(),
		&cancelResponse,
	)
	if err != nil {
		t.Fatalf("decode cancellation response: %v", err)
	}

	if cancelResponse.Order.ID != orderID {
		t.Fatalf(
			"expected order ID %s, got %s",
			orderID,
			cancelResponse.Order.ID,
		)
	}

	if cancelResponse.Order.Status != "CANCELLED" {
		t.Fatalf(
			"expected order status CANCELLED, got %s",
			cancelResponse.Order.Status,
		)
	}

	if len(cancelResponse.Reservations) != 1 {
		t.Fatalf(
			"expected 1 reservation, got %d",
			len(cancelResponse.Reservations),
		)
	}

	if cancelResponse.Reservations[0].Status != "CANCELLED" {
		t.Fatalf(
			"expected reservation status CANCELLED, got %s",
			cancelResponse.Reservations[0].Status,
		)
	}

	t.Cleanup(func() {
		_, _ = pool.Exec(
			ctx,
			`DELETE FROM payments WHERE order_id = $1`,
			orderID,
		)

		_, _ = pool.Exec(
			ctx,
			`DELETE FROM reservations WHERE order_id = $1`,
			orderID,
		)

		_, _ = pool.Exec(
			ctx,
			`DELETE FROM orders WHERE id = $1`,
			orderID,
		)
	})
}

func TestCancelOrderHandlerInvalidOrderID(t *testing.T) {
	handler, _, _, _ := setupCheckoutHandler(t)

	gin.SetMode(gin.TestMode)

	router := gin.New()

	router.POST(
		"/api/v1/orders/:id/cancel",
		handler.CancelOrder,
	)

	request := httptest.NewRequest(
		http.MethodPost,
		"/api/v1/orders/not-a-uuid/cancel",
		nil,
	)

	recorder := httptest.NewRecorder()

	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf(
			"expected status 400, got %d: %s",
			recorder.Code,
			recorder.Body.String(),
		)
	}

	var response struct {
		Error ErrorResponse `json:"error"`
	}

	err := json.Unmarshal(
		recorder.Body.Bytes(),
		&response,
	)
	if err != nil {
		t.Fatalf("decode error response: %v", err)
	}

	if response.Error.Code != "INVALID_ORDER_ID" {
		t.Fatalf(
			"expected error code INVALID_ORDER_ID, got %s",
			response.Error.Code,
		)
	}
}

func TestCancelOrderHandlerNotFound(t *testing.T) {
	handler, _, _, _ := setupCheckoutHandler(t)

	gin.SetMode(gin.TestMode)

	router := gin.New()

	router.POST(
		"/api/v1/orders/:id/cancel",
		handler.CancelOrder,
	)

	orderID := uuid.New()

	request := httptest.NewRequest(
		http.MethodPost,
		"/api/v1/orders/"+orderID.String()+"/cancel",
		nil,
	)

	recorder := httptest.NewRecorder()

	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusNotFound {
		t.Fatalf(
			"expected status 404, got %d: %s",
			recorder.Code,
			recorder.Body.String(),
		)
	}

	var response struct {
		Error ErrorResponse `json:"error"`
	}

	err := json.Unmarshal(
		recorder.Body.Bytes(),
		&response,
	)
	if err != nil {
		t.Fatalf("decode error response: %v", err)
	}

	if response.Error.Code != "ORDER_NOT_FOUND" {
		t.Fatalf(
			"expected error code ORDER_NOT_FOUND, got %s",
			response.Error.Code,
		)
	}
}

func TestCancelOrderHandlerConfirmedOrder(t *testing.T) {
	handler, queries, _, pool := setupCheckoutHandler(t)

	ctx := context.Background()

	orderRecord, err := queries.CreateOrder(
		ctx,
		db.CreateOrderParams{
			TotalAmount: 899900,
			Currency:    "INR",
		},
	)
	if err != nil {
		t.Fatalf("create order: %v", err)
	}

	t.Cleanup(func() {
		_, _ = pool.Exec(
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

	gin.SetMode(gin.TestMode)

	router := gin.New()

	router.POST(
		"/api/v1/orders/:id/cancel",
		handler.CancelOrder,
	)

	orderID := uuid.UUID(orderRecord.ID.Bytes)

	request := httptest.NewRequest(
		http.MethodPost,
		"/api/v1/orders/"+orderID.String()+"/cancel",
		nil,
	)

	recorder := httptest.NewRecorder()

	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusConflict {
		t.Fatalf(
			"expected status 409, got %d: %s",
			recorder.Code,
			recorder.Body.String(),
		)
	}

	var response struct {
		Error ErrorResponse `json:"error"`
	}

	err = json.Unmarshal(
		recorder.Body.Bytes(),
		&response,
	)
	if err != nil {
		t.Fatalf("decode error response: %v", err)
	}

	if response.Error.Code != "ORDER_CANNOT_BE_CANCELLED" {
		t.Fatalf(
			"expected error code ORDER_CANNOT_BE_CANCELLED, got %s",
			response.Error.Code,
		)
	}
}
