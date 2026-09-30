package main

import (
	"context"
	"log"
	"net/http"
	"os"

	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"

	"github.com/san-sp/Golang_E-Commerce_Project/internal/database"
	"github.com/san-sp/Golang_E-Commerce_Project/internal/database/db"
	apphttp "github.com/san-sp/Golang_E-Commerce_Project/internal/http"
	"github.com/san-sp/Golang_E-Commerce_Project/internal/payment"
)

func main() {
	err := godotenv.Load()
	if err != nil {
		log.Println("warning: .env file not loaded")
	}

	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		log.Fatal("DATABASE_URL is not set")
	}

	ctx := context.Background()

	pool, err := database.NewPostgres(
		ctx,
		databaseURL,
	)
	if err != nil {
		log.Fatal("failed to connect to PostgreSQL:", err)
	}
	defer pool.Close()

	queries := db.New(pool)

	paymentProvider := payment.NewMockProvider()

	paymentService := payment.NewService(
		pool,
		queries,
		paymentProvider,
	)

	handler := apphttp.NewHandler(paymentService)

	router := gin.New()

	api := router.Group("/api/v1")

	api.POST(
		"/payments/webhook",
		handler.PaymentWebhook,
	)

	server := &http.Server{
		Addr:    ":8080",
		Handler: router,
	}

	log.Println("HTTP server listening on :8080")

	err = server.ListenAndServe()
	if err != nil && err != http.ErrServerClosed {
		log.Fatal("HTTP server failed:", err)
	}
}
