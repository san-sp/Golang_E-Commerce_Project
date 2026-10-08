package brand

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/san-sp/Golang_E-Commerce_Project/internal/database/db"
)

var (
	ErrBrandNotFound        = errors.New("brand not found")
	ErrBrandNameEmpty       = errors.New("brand name is required")
	ErrBrandSlugEmpty       = errors.New("brand slug is required")
	ErrBrandSlugExists      = errors.New("brand slug already exists")
	ErrProductNotFound      = errors.New("product not found")
	ErrProductBrandNotFound = errors.New("product brand not found")
)

type Service struct {
	pool    *pgxpool.Pool
	queries *db.Queries
}

func uuidToPgUUID(id uuid.UUID) pgtype.UUID {
	return pgtype.UUID{
		Bytes: id,
		Valid: true,
	}
}

func NewService(pool *pgxpool.Pool, queries *db.Queries) *Service {
	return &Service{
		pool:    pool,
		queries: queries,
	}
}

func (s *Service) CreateBrand(
	ctx context.Context,
	name string,
	slug string,
) (db.Brand, error) {
	if name == "" {
		return db.Brand{}, ErrBrandNameEmpty
	}

	if slug == "" {
		return db.Brand{}, ErrBrandSlugEmpty
	}

	brand, err := s.queries.CreateBrand(ctx, db.CreateBrandParams{
		Name: name,
		Slug: slug,
	})
	if err != nil {
		return db.Brand{}, err
	}

	return brand, nil
}

func (s *Service) GetBrand(
	ctx context.Context,
	id uuid.UUID,
) (db.Brand, error) {
	brand, err := s.queries.GetBrand(ctx, uuidToPgUUID(id))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return db.Brand{}, ErrBrandNotFound
		}

		return db.Brand{}, err
	}

	return brand, nil
}

func (s *Service) GetBrandBySlug(
	ctx context.Context,
	slug string,
) (db.Brand, error) {
	if slug == "" {
		return db.Brand{}, ErrBrandSlugEmpty
	}

	brand, err := s.queries.GetBrandBySlug(ctx, slug)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return db.Brand{}, ErrBrandNotFound
		}

		return db.Brand{}, err
	}

	return brand, nil
}

func (s *Service) ListBrands(
	ctx context.Context,
) ([]db.Brand, error) {
	return s.queries.ListBrands(ctx)
}

func (s *Service) AssignProductBrand(
	ctx context.Context,
	productID uuid.UUID,
	brandID uuid.UUID,
) error {
	// Verify the product exists.
	_, err := s.queries.GetProduct(ctx, uuidToPgUUID(productID))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrProductNotFound
		}

		return err
	}

	// Verify the brand exists.
	_, err = s.queries.GetBrand(ctx, uuidToPgUUID(brandID))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrBrandNotFound
		}

		return err
	}

	return s.queries.UpdateProductBrand(ctx, db.UpdateProductBrandParams{
		ID:      uuidToPgUUID(productID),
		BrandID: uuidToPgUUID(brandID),
	})
}

func (s *Service) ClearProductBrand(
	ctx context.Context,
	productID uuid.UUID,
) error {
	_, err := s.queries.GetProduct(ctx, uuidToPgUUID(productID))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrProductNotFound
		}

		return err
	}

	return s.queries.ClearProductBrand(ctx, uuidToPgUUID(productID))
}

func (s *Service) GetProductBrand(
	ctx context.Context,
	productID uuid.UUID,
) (db.Brand, error) {
	brand, err := s.queries.GetProductBrand(
		ctx,
		uuidToPgUUID(productID),
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return db.Brand{}, ErrProductBrandNotFound
		}

		return db.Brand{}, err
	}

	return brand, nil
}

func (s *Service) ListBrandProducts(
	ctx context.Context,
	brandID uuid.UUID,
) ([]db.Product, error) {
	_, err := s.queries.GetBrand(ctx, uuidToPgUUID(brandID))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrBrandNotFound
		}

		return nil, err
	}

	return s.queries.ListBrandProducts(
		ctx,
		uuidToPgUUID(brandID),
	)
}
