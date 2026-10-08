package inventory

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/san-sp/Golang_E-Commerce_Project/internal/database/db"
)

var ErrInventoryNotFound = errors.New("inventory not found")

type Inventory struct {
	VariantID         uuid.UUID
	Quantity          int64
	ReservedQuantity  int64
	AvailableQuantity int64
}

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

func (s *Service) GetInventory(
	ctx context.Context,
	variantID uuid.UUID,
) (Inventory, error) {
	id := pgtype.UUID{
		Bytes: variantID,
		Valid: true,
	}

	row, err := s.queries.GetInventory(ctx, id)
	if err != nil {
		return Inventory{}, ErrInventoryNotFound
	}

	return Inventory{
		VariantID:         uuid.UUID(row.VariantID.Bytes),
		Quantity:          row.Quantity,
		ReservedQuantity:  row.ReservedQuantity,
		AvailableQuantity: row.AvailableQuantity,
	}, nil
}
