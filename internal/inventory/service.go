package inventory

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/san-sp/Golang_E-Commerce_Project/internal/database/db"
)

var ErrInventoryNotFound = errors.New("inventory not found")

type MovementType string

const (
	MovementTypeRestock          MovementType = "RESTOCK"
	MovementTypeDamage           MovementType = "DAMAGE"
	MovementTypeReturn           MovementType = "RETURN"
	MovementTypeManualAdjustment MovementType = "MANUAL_ADJUSTMENT"
)

var ErrInvalidMovement = errors.New("invalid inventory movement")
var ErrInsufficientInventory = errors.New("insufficient inventory")

type Inventory struct {
	VariantID         uuid.UUID
	Quantity          int64
	ReservedQuantity  int64
	AvailableQuantity int64
}

type InventoryMovement struct {
	ID            uuid.UUID
	VariantID     uuid.UUID
	MovementType  MovementType
	Quantity      int64
	ReferenceType *string
	ReferenceID   *uuid.UUID
	CreatedAt     time.Time
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

func (s *Service) ListInventoryMovements(
	ctx context.Context,
	variantID uuid.UUID,
	limit int32,
	offset int32,
) ([]InventoryMovement, error) {
	variant := pgtype.UUID{
		Bytes: variantID,
		Valid: true,
	}

	rows, err := s.queries.ListInventoryMovements(
		ctx,
		db.ListInventoryMovementsParams{
			VariantID: variant,
			Limit:     limit,
			Offset:    offset,
		},
	)
	if err != nil {
		return nil, fmt.Errorf("list inventory movements: %w", err)
	}

	movements := make([]InventoryMovement, 0, len(rows))

	for _, row := range rows {
		movement := InventoryMovement{
			ID:           uuid.UUID(row.ID.Bytes),
			VariantID:    uuid.UUID(row.VariantID.Bytes),
			MovementType: MovementType(row.MovementType),
			Quantity:     row.Quantity,
			CreatedAt:    row.CreatedAt.Time,
		}

		if row.ReferenceType.Valid {
			referenceType := row.ReferenceType.String
			movement.ReferenceType = &referenceType
		}

		if row.ReferenceID.Valid {
			referenceID := uuid.UUID(row.ReferenceID.Bytes)
			movement.ReferenceID = &referenceID
		}

		movements = append(movements, movement)
	}

	return movements, nil
}

func (s *Service) AdjustInventory(
	ctx context.Context,
	variantID uuid.UUID,
	delta int64,
	movementType MovementType,
	referenceType *string,
	referenceID *uuid.UUID,
) (Inventory, error) {
	if delta == 0 {
		return Inventory{}, ErrInvalidMovement
	}

	switch movementType {
	case MovementTypeRestock, MovementTypeReturn:
		if delta < 0 {
			return Inventory{}, ErrInvalidMovement
		}

	case MovementTypeDamage:
		if delta > 0 {
			return Inventory{}, ErrInvalidMovement
		}

	case MovementTypeManualAdjustment:
		// Manual adjustments may increase or decrease stock.

	default:
		return Inventory{}, ErrInvalidMovement
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return Inventory{}, fmt.Errorf("begin inventory adjustment: %w", err)
	}
	defer tx.Rollback(ctx)

	queries := s.queries.WithTx(tx)

	variant := pgtype.UUID{
		Bytes: variantID,
		Valid: true,
	}

	currentInventory, err := queries.GetInventoryForUpdate(ctx, variant)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Inventory{}, ErrInventoryNotFound
		}
		return Inventory{}, fmt.Errorf("lock inventory: %w", err)
	}

	reservedQuantity, err := queries.GetActiveReservedQuantity(ctx, variant)
	if err != nil {
		return Inventory{}, fmt.Errorf("get reserved quantity: %w", err)
	}

	newQuantity := currentInventory.Quantity + delta

	if newQuantity < 0 || newQuantity < reservedQuantity {
		return Inventory{}, ErrInsufficientInventory
	}

	_, err = queries.AdjustInventory(
		ctx,
		db.AdjustInventoryParams{
			VariantID: variant,
			Quantity:  delta,
		},
	)
	if err != nil {
		return Inventory{}, fmt.Errorf("adjust inventory: %w", err)
	}

	var referenceTypeValue pgtype.Text
	if referenceType != nil {
		referenceTypeValue = pgtype.Text{
			String: *referenceType,
			Valid:  true,
		}
	}

	var referenceIDValue pgtype.UUID
	if referenceID != nil {
		referenceIDValue = pgtype.UUID{
			Bytes: *referenceID,
			Valid: true,
		}
	}

	_, err = queries.CreateInventoryMovement(
		ctx,
		db.CreateInventoryMovementParams{
			VariantID:     variant,
			MovementType:  string(movementType),
			Quantity:      delta,
			ReferenceType: referenceTypeValue,
			ReferenceID:   referenceIDValue,
		},
	)
	if err != nil {
		return Inventory{}, fmt.Errorf("record inventory movement: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return Inventory{}, fmt.Errorf("commit inventory adjustment: %w", err)
	}

	return s.GetInventory(ctx, variantID)
}
