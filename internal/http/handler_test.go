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

	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"

	"github.com/san-sp/Golang_E-Commerce_Project/internal/database"
	"github.com/san-sp/Golang_E-Commerce_Project/internal/database/db"
	"github.com/san-sp/Golang_E-Commerce_Project/internal/reservation"
)

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
		reservationService,
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
		reservationService,
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
