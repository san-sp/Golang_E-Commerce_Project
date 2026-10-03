package reservation

import (
	"context"
	"fmt"
	"time"

	"github.com/san-sp/Golang_E-Commerce_Project/internal/database/db"
)

type Worker struct {
	queries   *db.Queries
	interval  time.Duration
	batchSize int32
}

func NewWorker(
	queries *db.Queries,
	interval time.Duration,
	batchSize int32,
) *Worker {
	return &Worker{
		queries:   queries,
		interval:  interval,
		batchSize: batchSize,
	}
}

func (w *Worker) ProcessOnce(ctx context.Context) error {
	reservations, err := w.queries.ExpireExpiredReservations(
		ctx,
		w.batchSize,
	)
	if err != nil {
		return fmt.Errorf("expire reservations: %w", err)
	}

	if len(reservations) > 0 {
		fmt.Printf(
			"expired %d reservations\n",
			len(reservations),
		)
	}

	return nil
}

func (w *Worker) Run(ctx context.Context) error {
	ticker := time.NewTicker(w.interval)
	defer ticker.Stop()

	for {
		if err := w.ProcessOnce(ctx); err != nil {
			fmt.Printf("reservation expiration error: %v\n", err)
		}

		select {
		case <-ticker.C:
			continue

		case <-ctx.Done():
			return nil
		}
	}
}
