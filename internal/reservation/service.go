package reservation

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/san-sp/Golang_E-Commerce_Project/internal/database/db"
)

var ErrInsufficientStock = errors.New("insufficient stock")

type Service struct {
	pool           *pgxpool.Pool
	queries        *db.Queries
	reservationTTL time.Duration
}

func NewService(
	pool *pgxpool.Pool,
	queries *db.Queries,
	reservationTTL time.Duration,
) *Service {
	return &Service{
		pool:           pool,
		queries:        queries,
		reservationTTL: reservationTTL,
	}
}

func (s *Service) CreateReservation(
	ctx context.Context,
	variantID pgtype.UUID,
	quantity int64,
) (db.Reservation, error) {
	if quantity <= 0 {
		return db.Reservation{}, fmt.Errorf("quantity must be greater than zero")
	}

	expiresAt := pgtype.Timestamptz{
		Time:  time.Now().Add(s.reservationTTL),
		Valid: true,
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return db.Reservation{}, fmt.Errorf("begin transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	txQueries := s.queries.WithTx(tx)

	inventory, err := txQueries.GetInventoryForUpdate(ctx, variantID)
	if err != nil {
		return db.Reservation{}, fmt.Errorf("get inventory: %w", err)
	}

	reservedQuantity, err := txQueries.GetActiveReservedQuantity(ctx, variantID)
	if err != nil {
		return db.Reservation{}, fmt.Errorf("get active reserved quantity: %w", err)
	}

	availableQuantity := inventory.Quantity - reservedQuantity

	if quantity > availableQuantity {
		return db.Reservation{}, fmt.Errorf(
			"%w: requested=%d available=%d",
			ErrInsufficientStock,
			quantity,
			availableQuantity,
		)
	}

	reservation, err := txQueries.CreateReservation(
		ctx,
		db.CreateReservationParams{
			VariantID: variantID,
			OrderID:   pgtype.UUID{},
			Quantity:  quantity,
			ExpiresAt: expiresAt,
		},
	)
	if err != nil {
		return db.Reservation{}, fmt.Errorf("create reservation: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return db.Reservation{}, fmt.Errorf("commit reservation transaction: %w", err)
	}

	return reservation, nil
}

func (s *Service) ConfirmReservation(
	ctx context.Context,
	reservationID pgtype.UUID,
) (db.Reservation, error) {
	reservation, err := s.queries.ConfirmReservation(
		ctx,
		reservationID,
	)
	if err != nil {
		return db.Reservation{}, fmt.Errorf(
			"confirm reservation: %w",
			err,
		)
	}

	return reservation, nil
}
