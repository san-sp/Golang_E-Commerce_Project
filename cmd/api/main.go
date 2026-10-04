package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"

	"github.com/san-sp/Golang_E-Commerce_Project/internal/cart"
	"github.com/san-sp/Golang_E-Commerce_Project/internal/database"
	"github.com/san-sp/Golang_E-Commerce_Project/internal/database/db"
	apphttp "github.com/san-sp/Golang_E-Commerce_Project/internal/http"
	"github.com/san-sp/Golang_E-Commerce_Project/internal/payment"
	"github.com/san-sp/Golang_E-Commerce_Project/internal/reservation"
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

	reservationService := reservation.NewService(
		pool,
		queries,
		15*time.Minute,
	)

	cartService := cart.NewService(
		queries,
	)

	handler := apphttp.NewHandler(
		paymentService,
		reservationService,
		cartService,
	)

	router := gin.New()

	router.Use(
		gin.Recovery(),
		apphttp.RequestIDMiddleware(),
		apphttp.LoggingMiddleware(),
	)

	api := router.Group("/api/v1")

	api.POST(
		"/payments",
		handler.CreatePayment,
	)

	api.POST(
		"/reservations",
		handler.CreateReservation,
	)

	api.POST(
		"/payments/webhook",
		handler.PaymentWebhook,
	)

	api.POST(
		"/carts",
		handler.CreateCart,
	)

	api.GET(
		"/carts/:id",
		handler.GetCart,
	)

	api.POST(
		"/carts/:id/items",
		handler.AddCartItem,
	)

	api.PATCH(
		"/carts/:id/items/:variant_id",
		handler.UpdateCartItem,
	)

	api.DELETE(
		"/carts/:id/items/:variant_id",
		handler.RemoveCartItem,
	)

	server := &http.Server{
		Addr:              ":8080",
		Handler:           router,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      10 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	signalCtx, stop := signal.NotifyContext(
		context.Background(),
		os.Interrupt,
		syscall.SIGTERM,
	)
	defer stop()

	go func() {
		log.Println("HTTP server listening on :8080")

		err := server.ListenAndServe()
		if err != nil && err != http.ErrServerClosed {
			log.Fatal("HTTP server failed:", err)
		}
	}()

	<-signalCtx.Done()

	log.Println("Shutdown signal received")

	shutdownCtx, cancel := context.WithTimeout(
		context.Background(),
		10*time.Second,
	)
	defer cancel()

	err = server.Shutdown(shutdownCtx)
	if err != nil {
		log.Fatal("HTTP server shutdown failed:", err)
	}

	log.Println("HTTP server shutdown completed")
}
