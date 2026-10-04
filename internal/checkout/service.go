package checkout

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/san-sp/Golang_E-Commerce_Project/internal/database/db"
	"github.com/san-sp/Golang_E-Commerce_Project/internal/order"
	"github.com/san-sp/Golang_E-Commerce_Project/internal/reservation"
)

var (
	ErrCartNotFound  = errors.New("cart not found")
	ErrCartNotActive = errors.New("cart is not active")
	ErrCartEmpty     = errors.New("cart is empty")
)

type Service struct {
	pool               *pgxpool.Pool
	queries            *db.Queries
	orderService       *order.Service
	reservationService *reservation.Service
}

func NewService(
	pool *pgxpool.Pool,
	queries *db.Queries,
	orderService *order.Service,
	reservationService *reservation.Service,
) *Service {
	return &Service{
		pool:               pool,
		queries:            queries,
		orderService:       orderService,
		reservationService: reservationService,
	}
}

type Result struct {
	Order        db.Order
	OrderItems   []db.OrderItem
	Reservations []db.Reservation
}

func (s *Service) Checkout(
	ctx context.Context,
	cartID pgtype.UUID,
	currency string,
) (Result, error) {
	if !cartID.Valid {
		return Result{}, errors.New("cart ID is required")
	}

	if currency == "" {
		return Result{}, errors.New("currency is required")
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return Result{}, fmt.Errorf("begin checkout transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	txQueries := s.queries.WithTx(tx)

	cart, err := txQueries.GetCart(ctx, cartID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Result{}, ErrCartNotFound
		}

		return Result{}, fmt.Errorf("get cart: %w", err)
	}

	if cart.Status != "ACTIVE" {
		return Result{}, ErrCartNotActive
	}

	cartItems, err := txQueries.GetCartItems(ctx, cartID)
	if err != nil {
		return Result{}, fmt.Errorf("get cart items: %w", err)
	}

	if len(cartItems) == 0 {
		return Result{}, ErrCartEmpty
	}

	orderInputs := make([]order.ItemInput, 0, len(cartItems))

	for _, cartItem := range cartItems {
		variant, err := txQueries.GetProductVariant(
			ctx,
			cartItem.VariantID,
		)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return Result{}, fmt.Errorf(
					"product variant not found: %w",
					err,
				)
			}

			return Result{}, fmt.Errorf(
				"get product variant: %w",
				err,
			)
		}

		orderInputs = append(orderInputs, order.ItemInput{
			VariantID: cartItem.VariantID,
			Quantity:  cartItem.Quantity,
			UnitPrice: variant.Price,
		})
	}

	createdOrder, orderItems, err := s.orderService.CreateOrderTx(
		ctx,
		tx,
		orderInputs,
		currency,
	)
	if err != nil {
		return Result{}, fmt.Errorf("create order: %w", err)
	}

	reservations := make([]db.Reservation, 0, len(cartItems))

	for _, cartItem := range cartItems {
		reservation, err := s.reservationService.CreateReservationTx(
			ctx,
			tx,
			cartItem.VariantID,
			createdOrder.ID,
			cartItem.Quantity,
		)
		if err != nil {
			return Result{}, fmt.Errorf(
				"create reservation: %w",
				err,
			)
		}

		reservations = append(reservations, reservation)
	}

	_, err = txQueries.ConvertCart(ctx, cartID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Result{}, ErrCartNotActive
		}

		return Result{}, fmt.Errorf(
			"convert cart: %w",
			err,
		)
	}

	if err := tx.Commit(ctx); err != nil {
		return Result{}, fmt.Errorf(
			"commit checkout transaction: %w",
			err,
		)
	}

	return Result{
		Order:        createdOrder,
		OrderItems:   orderItems,
		Reservations: reservations,
	}, nil
}
