package reservation

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/san-sp/Golang_E-Commerce_Project/internal/database/db"
)

type Service struct {
	pool    *pgxpool.Pool
	queries *db.Queries
}

func NewService(
	pool *pgxpool.Pool,
	queries *db.Queries,
) *Service {
	return &Service{
		pool:    pool,
		queries: queries,
	}
}

func (s *Service) CreateReservation(
	ctx context.Context,
	variantID pgtype.UUID,
	quantity int64,
	expiresAt pgtype.Timestamptz,
) (db.Reservation, error) {
	if quantity <= 0 {
		return db.Reservation{}, fmt.Errorf("quantity must be greater than zero")
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
			"insufficient stock: requested=%d available=%d",
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
