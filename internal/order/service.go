package order

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/san-sp/Golang_E-Commerce_Project/internal/database/db"
)

var (
	ErrOrderNotFound = errors.New("order not found")
)

type Service struct {
	pool    *pgxpool.Pool
	queries *db.Queries
}

func NewService(pool *pgxpool.Pool, queries *db.Queries) *Service {
	return &Service{
		pool:    pool,
		queries: queries,
	}
}

type ItemInput struct {
	VariantID pgtype.UUID
	Quantity  int64
	UnitPrice int64
}

func (s *Service) CreateOrder(
	ctx context.Context,
	items []ItemInput,
	currency string,
) (db.Order, []db.OrderItem, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return db.Order{}, nil, err
	}
	defer tx.Rollback(ctx)

	order, orderItems, err := s.CreateOrderTx(
		ctx,
		tx,
		items,
		currency,
	)
	if err != nil {
		return db.Order{}, nil, err
	}

	if err := tx.Commit(ctx); err != nil {
		return db.Order{}, nil, err
	}

	return order, orderItems, nil
}

func (s *Service) CreateOrderTx(
	ctx context.Context,
	tx pgx.Tx,
	items []ItemInput,
	currency string,
) (db.Order, []db.OrderItem, error) {
	if len(items) == 0 {
		return db.Order{}, nil, errors.New(
			"order must contain at least one item",
		)
	}

	if currency == "" {
		return db.Order{}, nil, errors.New(
			"currency is required",
		)
	}

	var totalAmount int64

	for _, item := range items {
		if !item.VariantID.Valid {
			return db.Order{}, nil, errors.New(
				"variant ID is required",
			)
		}

		if item.Quantity <= 0 {
			return db.Order{}, nil, errors.New(
				"quantity must be greater than zero",
			)
		}

		if item.UnitPrice < 0 {
			return db.Order{}, nil, errors.New(
				"unit price cannot be negative",
			)
		}

		totalAmount += item.Quantity * item.UnitPrice
	}

	txQueries := s.queries.WithTx(tx)

	order, err := txQueries.CreateOrder(
		ctx,
		db.CreateOrderParams{
			TotalAmount: totalAmount,
			Currency:    currency,
		},
	)
	if err != nil {
		return db.Order{}, nil, err
	}

	orderItems := make([]db.OrderItem, 0, len(items))

	for _, item := range items {
		orderItem, err := txQueries.CreateOrderItem(
			ctx,
			db.CreateOrderItemParams{
				OrderID:   order.ID,
				VariantID: item.VariantID,
				Quantity:  item.Quantity,
				UnitPrice: item.UnitPrice,
			},
		)
		if err != nil {
			return db.Order{}, nil, err
		}

		orderItems = append(orderItems, orderItem)
	}

	return order, orderItems, nil
}

func (s *Service) GetOrder(
	ctx context.Context,
	orderID uuid.UUID,
) (db.Order, []db.OrderItem, error) {
	orderUUID := pgtype.UUID{
		Bytes: orderID,
		Valid: true,
	}

	order, err := s.queries.GetOrder(ctx, orderUUID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return db.Order{}, nil, ErrOrderNotFound
		}

		return db.Order{}, nil, err
	}

	items, err := s.queries.GetOrderItems(ctx, orderUUID)
	if err != nil {
		return db.Order{}, nil, err
	}

	return order, items, nil
}

func (s *Service) CancelOrder(
	ctx context.Context,
	orderID pgtype.UUID,
) (db.Order, error) {
	order, err := s.queries.CancelOrder(
		ctx,
		orderID,
	)
	if err != nil {
		return db.Order{}, fmt.Errorf(
			"cancel order: %w",
			err,
		)
	}

	return order, nil
}
