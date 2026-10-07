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

-- name: ListProducts :many
SELECT
    id,
    name,
    description,
    created_at,
    updated_at
FROM products
ORDER BY created_at DESC
LIMIT $1
OFFSET $2;    

-- name: GetProduct :one
SELECT
    id,
    name,
    description,
    created_at,
    updated_at
FROM products
WHERE id = $1;

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
