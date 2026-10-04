package cart

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/san-sp/Golang_E-Commerce_Project/internal/database/db"
)

var (
	ErrCartNotFound     = errors.New("cart not found")
	ErrCartNotActive    = errors.New("cart is not active")
	ErrInvalidQuantity  = errors.New("quantity must be greater than zero")
	ErrCartItemNotFound = errors.New("cart item not found")
)

type Service struct {
	queries *db.Queries
}

func NewService(queries *db.Queries) *Service {
	return &Service{
		queries: queries,
	}
}

func (s *Service) CreateCart(
	ctx context.Context,
) (db.Cart, error) {
	cart, err := s.queries.CreateCart(ctx)
	if err != nil {
		return db.Cart{}, fmt.Errorf("create cart: %w", err)
	}

	return cart, nil
}

func (s *Service) GetCart(
	ctx context.Context,
	cartID pgtype.UUID,
) (db.Cart, error) {
	cart, err := s.queries.GetCart(ctx, cartID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return db.Cart{}, ErrCartNotFound
		}

		return db.Cart{}, fmt.Errorf("get cart: %w", err)
	}

	return cart, nil
}

func (s *Service) GetCartItems(
	ctx context.Context,
	cartID pgtype.UUID,
) ([]db.CartItem, error) {
	_, err := s.queries.GetCart(ctx, cartID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrCartNotFound
		}

		return nil, fmt.Errorf("get cart: %w", err)
	}

	items, err := s.queries.GetCartItems(ctx, cartID)
	if err != nil {
		return nil, fmt.Errorf("get cart items: %w", err)
	}

	return items, nil
}

func (s *Service) AddItem(
	ctx context.Context,
	cartID pgtype.UUID,
	variantID pgtype.UUID,
	quantity int64,
) (db.CartItem, error) {
	if quantity <= 0 {
		return db.CartItem{}, ErrInvalidQuantity
	}

	cart, err := s.queries.GetCart(ctx, cartID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return db.CartItem{}, ErrCartNotFound
		}

		return db.CartItem{}, fmt.Errorf("get cart: %w", err)
	}

	if cart.Status != "ACTIVE" {
		return db.CartItem{}, ErrCartNotActive
	}

	item, err := s.queries.AddCartItem(
		ctx,
		db.AddCartItemParams{
			CartID:    cartID,
			VariantID: variantID,
			Quantity:  quantity,
		},
	)
	if err != nil {
		return db.CartItem{}, fmt.Errorf("add cart item: %w", err)
	}

	return item, nil
}

func (s *Service) UpdateItemQuantity(
	ctx context.Context,
	cartID pgtype.UUID,
	variantID pgtype.UUID,
	quantity int64,
) (db.CartItem, error) {
	if quantity <= 0 {
		return db.CartItem{}, ErrInvalidQuantity
	}

	cart, err := s.queries.GetCart(ctx, cartID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return db.CartItem{}, ErrCartNotFound
		}

		return db.CartItem{}, fmt.Errorf("get cart: %w", err)
	}

	if cart.Status != "ACTIVE" {
		return db.CartItem{}, ErrCartNotActive
	}

	item, err := s.queries.UpdateCartItemQuantity(
		ctx,
		db.UpdateCartItemQuantityParams{
			CartID:    cartID,
			VariantID: variantID,
			Quantity:  quantity,
		},
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return db.CartItem{}, ErrCartItemNotFound
		}

		return db.CartItem{}, fmt.Errorf(
			"update cart item quantity: %w",
			err,
		)
	}

	return item, nil
}

func (s *Service) RemoveItem(
	ctx context.Context,
	cartID pgtype.UUID,
	variantID pgtype.UUID,
) error {
	cart, err := s.queries.GetCart(ctx, cartID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrCartNotFound
		}

		return fmt.Errorf("get cart: %w", err)
	}

	if cart.Status != "ACTIVE" {
		return ErrCartNotActive
	}

	err = s.queries.RemoveCartItem(
		ctx,
		db.RemoveCartItemParams{
			CartID:    cartID,
			VariantID: variantID,
		},
	)
	if err != nil {
		return fmt.Errorf("remove cart item: %w", err)
	}

	return nil
}
