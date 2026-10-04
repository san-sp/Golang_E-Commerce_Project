-- name: CreateCart :one
INSERT INTO carts (
    status
)
VALUES (
    'ACTIVE'
)
RETURNING
    id,
    status,
    created_at,
    updated_at;


-- name: GetCart :one
SELECT
    id,
    status,
    created_at,
    updated_at
FROM carts
WHERE id = $1;


-- name: GetActiveCart :one
SELECT
    id,
    status,
    created_at,
    updated_at
FROM carts
WHERE id = $1
  AND status = 'ACTIVE';


-- name: AddCartItem :one
INSERT INTO cart_items (
    cart_id,
    variant_id,
    quantity
)
VALUES (
    $1,
    $2,
    $3
)
ON CONFLICT (cart_id, variant_id)
DO UPDATE
SET
    quantity = cart_items.quantity + EXCLUDED.quantity,
    updated_at = NOW()
RETURNING
    id,
    cart_id,
    variant_id,
    quantity,
    created_at,
    updated_at;


-- name: GetCartItem :one
SELECT
    id,
    cart_id,
    variant_id,
    quantity,
    created_at,
    updated_at
FROM cart_items
WHERE cart_id = $1
  AND variant_id = $2;


-- name: GetCartItems :many
SELECT
    id,
    cart_id,
    variant_id,
    quantity,
    created_at,
    updated_at
FROM cart_items
WHERE cart_id = $1
ORDER BY created_at;


-- name: UpdateCartItemQuantity :one
UPDATE cart_items
SET
    quantity = $3,
    updated_at = NOW()
WHERE cart_id = $1
  AND variant_id = $2
RETURNING
    id,
    cart_id,
    variant_id,
    quantity,
    created_at,
    updated_at;


-- name: RemoveCartItem :exec
DELETE FROM cart_items
WHERE cart_id = $1
  AND variant_id = $2;


-- name: ConvertCart :one
UPDATE carts
SET
    status = 'CONVERTED',
    updated_at = NOW()
WHERE id = $1
  AND status = 'ACTIVE'
RETURNING
    id,
    status,
    created_at,
    updated_at;