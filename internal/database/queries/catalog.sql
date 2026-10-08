-- name: CreateProduct :one
INSERT INTO products (
    name,
    description
)
VALUES ($1, $2)
RETURNING
    id,
    name,
    description,
    created_at,
    updated_at;

-- name: ListProducts :many
SELECT
    id,
    name,
    description,
    created_at,
    updated_at,
    brand_id
FROM products
ORDER BY created_at DESC, id DESC
LIMIT $1
OFFSET $2;    

-- name: ListProductsAsc :many
SELECT
    id,
    name,
    description,
    created_at,
    updated_at,
    brand_id
FROM products
ORDER BY created_at ASC, id ASC
LIMIT $1
OFFSET $2;

-- name: SearchProducts :many
SELECT
    id,
    name,
    description,
    created_at,
    updated_at,
    brand_id
FROM products
WHERE name ILIKE '%' || sqlc.arg(search_term) || '%'
ORDER BY created_at DESC, id DESC
LIMIT sqlc.arg(page_limit)
OFFSET sqlc.arg(page_offset);

-- name: ListProductsMinPrice :many
SELECT
    p.id,
    p.name,
    p.description,
    p.created_at,
    p.updated_at,
    p.brand_id
FROM products p
JOIN product_variants pv
    ON pv.product_id = p.id
GROUP BY
    p.id,
    p.name,
    p.description,
    p.brand_id,
    p.created_at,
    p.updated_at
HAVING MIN(pv.price) >= sqlc.arg(min_price)
ORDER BY p.created_at DESC, p.id DESC
LIMIT sqlc.arg(page_limit)
OFFSET sqlc.arg(page_offset);

-- name: ListProductsPriceAsc :many
SELECT
    p.id,
    p.name,
    p.description,
    p.created_at,
    p.updated_at,
    p.brand_id
FROM products p
JOIN product_variants pv
    ON pv.product_id = p.id
GROUP BY
    p.id,
    p.name,
    p.description,
    p.brand_id,
    p.created_at,
    p.updated_at
ORDER BY MIN(pv.price) ASC, p.id ASC
LIMIT $1
OFFSET $2;

-- name: ListProductsPriceDesc :many
SELECT
    p.id,
    p.name,
    p.description,
    p.created_at,
    p.updated_at,
    p.brand_id
FROM products p
JOIN product_variants pv
    ON pv.product_id = p.id
GROUP BY
    p.id,
    p.name,
    p.description,
    p.brand_id,
    p.created_at,
    p.updated_at
ORDER BY MIN(pv.price) DESC, p.id DESC
LIMIT $1
OFFSET $2;

-- name: GetProduct :one
SELECT
    id,
    name,
    description,
    created_at,
    updated_at,
    brand_id
FROM products
WHERE id = $1;

-- name: CreateProductVariant :one
INSERT INTO product_variants (
    product_id,
    sku,
    size,
    color,
    price
)
VALUES ($1, $2, $3, $4, $5)
RETURNING
    id,
    product_id,
    sku,
    size,
    color,
    price,
    created_at,
    updated_at;

-- name: ListProductVariants :many
SELECT
    id,
    product_id,
    sku,
    size,
    color,
    price,
    created_at,
    updated_at
FROM product_variants
WHERE product_id = $1
ORDER BY created_at ASC;
  
-- name: GetProductVariant :one
SELECT
    id,
    product_id,
    sku,
    size,
    color,
    price,
    created_at,
    updated_at
FROM product_variants
WHERE id = $1;

-- name: CreateCategory :one
INSERT INTO categories (
    name,
    slug
)
VALUES ($1, $2)
RETURNING
    id,
    name,
    slug,
    created_at,
    updated_at;

-- name: GetCategory :one
SELECT
    id,
    name,
    slug,
    created_at,
    updated_at
FROM categories
WHERE id = $1;

-- name: GetCategoryBySlug :one
SELECT
    id,
    name,
    slug,
    created_at,
    updated_at
FROM categories
WHERE slug = $1;

-- name: ListCategories :many
SELECT
    id,
    name,
    slug,
    created_at,
    updated_at
FROM categories
ORDER BY created_at DESC, id DESC;

-- name: AddProductToCategory :exec
INSERT INTO product_categories (
    product_id,
    category_id
)
VALUES ($1, $2);

-- name: RemoveProductFromCategory :exec
DELETE FROM product_categories
WHERE product_id = $1
  AND category_id = $2;

-- name: ListProductCategories :many
SELECT
    c.id,
    c.name,
    c.slug,
    c.created_at,
    c.updated_at
FROM categories c
JOIN product_categories pc
    ON pc.category_id = c.id
WHERE pc.product_id = $1
ORDER BY c.created_at DESC, c.id DESC;

-- name: ListCategoryProducts :many
SELECT
    p.id,
    p.name,
    p.description,
    p.created_at,
    p.updated_at,
    p.brand_id
FROM products p
JOIN product_categories pc
    ON pc.product_id = p.id
WHERE pc.category_id = $1
ORDER BY p.created_at DESC, p.id DESC;

-- name: CreateBrand :one
INSERT INTO brands (name, slug)
VALUES ($1, $2)
RETURNING id, name, slug, created_at, updated_at;

-- name: GetBrand :one
SELECT id, name, slug, created_at, updated_at
FROM brands
WHERE id = $1;

-- name: GetBrandBySlug :one
SELECT id, name, slug, created_at, updated_at
FROM brands
WHERE slug = $1;

-- name: ListBrands :many
SELECT id, name, slug, created_at, updated_at
FROM brands
ORDER BY created_at DESC, id DESC;

-- name: UpdateProductBrand :exec
UPDATE products
SET brand_id = $2,
    updated_at = NOW()
WHERE id = $1;

-- name: ClearProductBrand :exec
UPDATE products
SET brand_id = NULL,
    updated_at = NOW()
WHERE id = $1;

-- name: GetProductBrand :one
SELECT b.id, b.name, b.slug, b.created_at, b.updated_at
FROM brands b
JOIN products p ON p.brand_id = b.id
WHERE p.id = $1;

-- name: ListBrandProducts :many
SELECT
    p.id,
    p.name,
    p.description,
    p.created_at,
    p.updated_at,
    p.brand_id
FROM products p
WHERE p.brand_id = $1
ORDER BY p.created_at DESC, p.id DESC;