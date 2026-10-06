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


-- name: ConsumeInventory :one
UPDATE inventory
SET
    quantity = quantity - $2,
    updated_at = NOW()
WHERE variant_id = $1
  AND quantity >= $2
RETURNING
    id,
    variant_id,
    quantity,
    created_at,
    updated_at;


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

-- name: CancelReservation :one
UPDATE reservations
SET
    status = 'CANCELLED',
    updated_at = NOW()
WHERE id = $1
  AND status = 'ACTIVE'
RETURNING
    id,
    variant_id,
    order_id,
    quantity,
    status,
    expires_at,
    created_at,
    updated_at;    

-- name: ExpireReservation :one
UPDATE reservations
SET
    status = 'EXPIRED',
    updated_at = NOW()
WHERE id = $1
  AND status = 'ACTIVE'
  AND expires_at <= NOW()
RETURNING
    id,
    variant_id,
    order_id,
    quantity,
    status,
    expires_at,
    created_at,
    updated_at;

-- name: ExpireExpiredReservations :many
WITH expired AS (
    SELECT id
    FROM reservations
    WHERE status = 'ACTIVE'
      AND expires_at <= NOW()
    ORDER BY expires_at
    LIMIT $1
    FOR UPDATE SKIP LOCKED
)
UPDATE reservations AS r
SET
    status = 'EXPIRED',
    updated_at = NOW()
FROM expired
WHERE r.id = expired.id
RETURNING
    r.id,
    r.variant_id,
    r.order_id,
    r.quantity,
    r.status,
    r.expires_at,
    r.created_at,
    r.updated_at;

-- name: GetReservation :one
SELECT
    id,
    variant_id,
    order_id,
    quantity,
    status,
    expires_at,
    created_at,
    updated_at
FROM reservations
WHERE id = $1;

-- name: GetActiveReservationsByOrderID :many
SELECT
    id,
    variant_id,
    order_id,
    quantity,
    status,
    expires_at,
    created_at,
    updated_at
FROM reservations
WHERE order_id = $1
  AND status = 'ACTIVE'
ORDER BY created_at;

-- name: GetReservationsByOrderID :many
SELECT
    id,
    variant_id,
    order_id,
    quantity,
    status,
    expires_at,
    created_at,
    updated_at
FROM reservations
WHERE order_id = $1
ORDER BY created_at;