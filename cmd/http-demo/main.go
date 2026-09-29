package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"time"
)

func paymentHandler(
	w http.ResponseWriter,
	r *http.Request,
) {
	fmt.Println("Payment request started")

	time.Sleep(5 * time.Second)

	fmt.Fprintln(w, "Payment request completed")

	fmt.Println("Payment request completed")
}

func main() {
	http.HandleFunc(
		"/payments",
		paymentHandler,
	)

	server := &http.Server{
		Addr:              ":8080",
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      10 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	stop := make(chan os.Signal, 1)

	signal.Notify(
		stop,
		os.Interrupt,
	)

	go func() {
		<-stop

		fmt.Println("Shutdown signal received")

		ctx, cancel := context.WithTimeout(
			context.Background(),
			10*time.Second,
		)
		defer cancel()

		err := server.Shutdown(ctx)
		if err != nil {
			fmt.Println("Graceful shutdown failed:", err)
			return
		}

		fmt.Println("Server shutdown completed")
	}()

	fmt.Println("HTTP server listening on :8080")

	err := server.ListenAndServe()
	if err != nil && err != http.ErrServerClosed {
		fmt.Println("HTTP server failed:", err)
	}
}
