-- name: CreateOrder :one
INSERT INTO orders (
    status,
    total_amount,
    currency
)
VALUES (
    'PENDING',
    $1,
    $2
)
RETURNING
    id,
    status,
    total_amount,
    currency,
    created_at,
    updated_at;


-- name: GetOrder :one
SELECT
    id,
    status,
    total_amount,
    currency,
    created_at,
    updated_at
FROM orders
WHERE id = $1;


-- name: ConfirmOrder :one
UPDATE orders
SET
    status = 'CONFIRMED',
    updated_at = NOW()
WHERE id = $1
  AND status = 'PENDING'
RETURNING
    id,
    status,
    total_amount,
    currency,
    created_at,
    updated_at;


-- name: RefundOrder :one
UPDATE orders
SET
    status = 'REFUNDED',
    updated_at = NOW()
WHERE id = $1
  AND status = 'CONFIRMED'
RETURNING
    id,
    status,
    total_amount,
    currency,
    created_at,
    updated_at;    


-- name: CancelOrder :one
UPDATE orders
SET
    status = 'CANCELLED',
    updated_at = NOW()
WHERE id = $1
  AND status = 'PENDING'
RETURNING
    id,
    status,
    total_amount,
    currency,
    created_at,
    updated_at;


-- name: GetOrderItems :many
SELECT
    id,
    order_id,
    variant_id,
    quantity,
    unit_price,
    created_at,
    updated_at
FROM order_items
WHERE order_id = $1
ORDER BY created_at;


-- name: CreateOrderItem :one
INSERT INTO order_items (
    order_id,
    variant_id,
    quantity,
    unit_price
)
VALUES (
    $1,
    $2,
    $3,
    $4
)
RETURNING
    id,
    order_id,
    variant_id,
    quantity,
    unit_price,
    created_at,
    updated_at;