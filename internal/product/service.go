package product

import (
	"context"
	"errors"
	"strings"

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
	ErrInvalidProductSort     = errors.New("invalid product sort")
	ErrInvalidProductSearch   = errors.New("search term cannot be empty")
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
	sort string,
) ([]db.Product, error) {
	if page < 1 {
		return nil, ErrInvalidPage
	}

	if limit < 1 || limit > 100 {
		return nil, ErrInvalidLimit
	}

	if sort == "" {
		sort = "created_desc"
	}

	offset := (page - 1) * limit

	var (
		products []db.Product
		err      error
	)

	switch sort {
	case "created_desc":
		products, err = s.queries.ListProducts(ctx, db.ListProductsParams{
			Limit:  int32(limit),
			Offset: int32(offset),
		})

	case "created_asc":
		products, err = s.queries.ListProductsAsc(ctx, db.ListProductsAscParams{
			Limit:  int32(limit),
			Offset: int32(offset),
		})

	case "price_asc":
		products, err = s.queries.ListProductsPriceAsc(ctx, db.ListProductsPriceAscParams{
			Limit:  int32(limit),
			Offset: int32(offset),
		})

	case "price_desc":
		products, err = s.queries.ListProductsPriceDesc(ctx, db.ListProductsPriceDescParams{
			Limit:  int32(limit),
			Offset: int32(offset),
		})

	default:
		return nil, ErrInvalidProductSort
	}

	if err != nil {
		return nil, err
	}

	if products == nil {
		products = []db.Product{}
	}

	return products, nil
}

func (s *Service) SearchProducts(
	ctx context.Context,
	search string,
	page int,
	limit int,
) ([]db.Product, error) {
	if page < 1 {
		return nil, ErrInvalidPage
	}

	if limit < 1 || limit > 100 {
		return nil, ErrInvalidLimit
	}

	search = strings.TrimSpace(search)

	if search == "" {
		return nil, ErrInvalidProductSearch
	}

	offset := (page - 1) * limit

	products, err := s.queries.SearchProducts(
		ctx,
		db.SearchProductsParams{
			SearchTerm: pgtype.Text{
				String: search,
				Valid:  true,
			},
			PageLimit:  int32(limit),
			PageOffset: int32(offset),
		},
	)
	if err != nil {
		return nil, err
	}

	if products == nil {
		products = []db.Product{}
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

func (s *Service) ListProductVariants(
	ctx context.Context,
	productID uuid.UUID,
) ([]db.ProductVariant, error) {
	variantProductID := pgtype.UUID{
		Bytes: productID,
		Valid: true,
	}

	variants, err := s.queries.ListProductVariants(
		ctx,
		variantProductID,
	)
	if err != nil {
		return nil, err
	}

	if variants == nil {
		variants = []db.ProductVariant{}
	}

	return variants, nil
}
