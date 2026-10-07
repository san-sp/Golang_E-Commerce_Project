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
    updated_at
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
    updated_at
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
    updated_at
FROM products
WHERE name ILIKE '%' || sqlc.arg(search_term) || '%'
ORDER BY created_at DESC, id DESC
LIMIT sqlc.arg(page_limit)
OFFSET sqlc.arg(page_offset);

-- name: ListProductsPriceAsc :many
SELECT
    p.id,
    p.name,
    p.description,
    p.created_at,
    p.updated_at
FROM products p
JOIN product_variants pv
    ON pv.product_id = p.id
GROUP BY
    p.id,
    p.name,
    p.description,
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
    p.updated_at
FROM products p
JOIN product_variants pv
    ON pv.product_id = p.id
GROUP BY
    p.id,
    p.name,
    p.description,
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
    updated_at
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
