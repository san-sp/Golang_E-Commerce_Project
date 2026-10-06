-- name: CreatePayment :one
INSERT INTO payments (
    order_id,
    provider,
    provider_payment_id,
    amount,
    currency,
    status
)
VALUES (
    $1,
    $2,
    $3,
    $4,
    $5,
    $6
)
RETURNING *;

-- name: GetPayment :one
SELECT
    id,
    reservation_id,
    provider,
    provider_payment_id,
    amount,
    currency,
    status,
    created_at,
    updated_at,
    order_id
FROM payments
WHERE id = $1;

-- name: UpdatePaymentProviderID :one
UPDATE payments
SET
    provider_payment_id = $2,
    updated_at = NOW()
WHERE id = $1
RETURNING *;

-- name: MarkPaymentFailed :one
UPDATE payments
SET
    status = 'FAILED',
    updated_at = NOW()
WHERE id = $1
RETURNING *;

-- name: MarkPaymentSucceeded :one
UPDATE payments
SET
    status = 'SUCCEEDED',
    updated_at = NOW()
WHERE id = $1
  AND status = 'PENDING'
RETURNING *;
