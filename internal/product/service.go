package product

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/san-sp/Golang_E-Commerce_Project/internal/database/db"
)

var (
	ErrProductNotFound        = errors.New("product not found")
	ErrProductVariantNotFound = errors.New("product variant not found")
	ErrInvalidPage            = errors.New("page must be greater than zero")
	ErrInvalidLimit           = errors.New("limit must be between 1 and 100")
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

func (s *Service) GetProduct(ctx context.Context, id uuid.UUID) (db.Product, error) {
	productID := pgtype.UUID{
		Bytes: id,
		Valid: true,
	}

	product, err := s.queries.GetProduct(ctx, productID)
	if err != nil {
		return db.Product{}, ErrProductNotFound
	}

	return product, nil
}

func (s *Service) ListProducts(
	ctx context.Context,
	page int,
	limit int,
) ([]db.Product, error) {
	if page < 1 {
		return nil, ErrInvalidPage
	}

	if limit < 1 || limit > 100 {
		return nil, ErrInvalidLimit
	}

	offset := (page - 1) * limit

	products, err := s.queries.ListProducts(
		ctx,
		db.ListProductsParams{
			Limit:  int32(limit),
			Offset: int32(offset),
		},
	)
	if err != nil {
		return nil, err
	}

	return products, nil
}

func (s *Service) GetProductVariant(ctx context.Context, id uuid.UUID) (db.ProductVariant, error) {
	variantID := pgtype.UUID{
		Bytes: id,
		Valid: true,
	}

	variant, err := s.queries.GetProductVariant(ctx, variantID)
	if err != nil {
		return db.ProductVariant{}, ErrProductVariantNotFound
	}

	return variant, nil
}
