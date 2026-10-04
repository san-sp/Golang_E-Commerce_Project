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
