-- name: GetInventoryForUpdate :one
SELECT
    id,
    variant_id,
    quantity,
    created_at,
    updated_at
FROM inventory
WHERE variant_id = $1
FOR UPDATE;


-- name: GetActiveReservedQuantity :one
SELECT
    COALESCE(SUM(quantity), 0)::BIGINT AS reserved_quantity
FROM reservations
WHERE variant_id = $1
  AND status = 'ACTIVE'
  AND expires_at > NOW();


-- name: CreateReservation :one
INSERT INTO reservations (
    variant_id,
    order_id,
    quantity,
    status,
    expires_at
)
VALUES (
    $1,
    $2,
    $3,
    'ACTIVE',
    $4
)
RETURNING
    id,
    variant_id,
    order_id,
    quantity,
    status,
    expires_at,
    created_at,
    updated_at;

-- name: ConfirmReservation :one
UPDATE reservations
SET
    status = 'CONFIRMED',
    updated_at = NOW()
WHERE id = $1
  AND status = 'ACTIVE'
  AND expires_at > NOW()
RETURNING
    id,
    variant_id,
    order_id,
    quantity,
    status,
    expires_at,
    created_at,
    updated_at;  