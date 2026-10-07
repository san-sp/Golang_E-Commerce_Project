package category

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
	ErrCategoryNotFound        = errors.New("category not found")
	ErrCategorySlugExists      = errors.New("category slug already exists")
	ErrCategoryNameEmpty       = errors.New("category name cannot be empty")
	ErrCategorySlugEmpty       = errors.New("category slug cannot be empty")
	ErrProductCategoryExists   = errors.New("product is already assigned to this category")
	ErrProductCategoryNotFound = errors.New("product is not assigned to this category")
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

func (s *Service) CreateCategory(
	ctx context.Context,
	name string,
	slug string,
) (db.Category, error) {
	name = strings.TrimSpace(name)
	slug = strings.TrimSpace(slug)

	if name == "" {
		return db.Category{}, ErrCategoryNameEmpty
	}

	if slug == "" {
		return db.Category{}, ErrCategorySlugEmpty
	}

	category, err := s.queries.CreateCategory(
		ctx,
		db.CreateCategoryParams{
			Name: name,
			Slug: slug,
		},
	)
	if err != nil {
		return db.Category{}, err
	}

	return category, nil
}

func (s *Service) GetCategory(
	ctx context.Context,
	id uuid.UUID,
) (db.Category, error) {
	categoryID := pgtype.UUID{
		Bytes: id,
		Valid: true,
	}

	category, err := s.queries.GetCategory(ctx, categoryID)
	if err != nil {
		return db.Category{}, ErrCategoryNotFound
	}

	return category, nil
}

func (s *Service) GetCategoryBySlug(
	ctx context.Context,
	slug string,
) (db.Category, error) {
	slug = strings.TrimSpace(slug)

	if slug == "" {
		return db.Category{}, ErrCategorySlugEmpty
	}

	category, err := s.queries.GetCategoryBySlug(ctx, slug)
	if err != nil {
		return db.Category{}, ErrCategoryNotFound
	}

	return category, nil
}

func (s *Service) ListCategories(
	ctx context.Context,
) ([]db.Category, error) {
	categories, err := s.queries.ListCategories(ctx)
	if err != nil {
		return nil, err
	}

	if categories == nil {
		categories = []db.Category{}
	}

	return categories, nil
}

func (s *Service) AddProductToCategory(
	ctx context.Context,
	productID uuid.UUID,
	categoryID uuid.UUID,
) error {
	err := s.queries.AddProductToCategory(
		ctx,
		db.AddProductToCategoryParams{
			ProductID: pgtype.UUID{
				Bytes: productID,
				Valid: true,
			},
			CategoryID: pgtype.UUID{
				Bytes: categoryID,
				Valid: true,
			},
		},
	)
	if err != nil {
		return err
	}

	return nil
}

func (s *Service) RemoveProductFromCategory(
	ctx context.Context,
	productID uuid.UUID,
	categoryID uuid.UUID,
) error {
	err := s.queries.RemoveProductFromCategory(
		ctx,
		db.RemoveProductFromCategoryParams{
			ProductID: pgtype.UUID{
				Bytes: productID,
				Valid: true,
			},
			CategoryID: pgtype.UUID{
				Bytes: categoryID,
				Valid: true,
			},
		},
	)
	if err != nil {
		return err
	}

	return nil
}

func (s *Service) ListProductCategories(
	ctx context.Context,
	productID uuid.UUID,
) ([]db.Category, error) {
	categories, err := s.queries.ListProductCategories(
		ctx,
		pgtype.UUID{
			Bytes: productID,
			Valid: true,
		},
	)
	if err != nil {
		return nil, err
	}

	if categories == nil {
		categories = []db.Category{}
	}

	return categories, nil
}

func (s *Service) ListCategoryProducts(
	ctx context.Context,
	categoryID uuid.UUID,
) ([]db.Product, error) {
	products, err := s.queries.ListCategoryProducts(
		ctx,
		pgtype.UUID{
			Bytes: categoryID,
			Valid: true,
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
