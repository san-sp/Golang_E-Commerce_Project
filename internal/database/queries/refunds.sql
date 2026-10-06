-- name: CreateRefund :one
INSERT INTO refunds (
    payment_id,
    amount,
    currency,
    status,
    idempotency_key
)
VALUES (
    $1,
    $2,
    $3,
    'PENDING',
    $4
)
RETURNING *;


-- name: GetRefund :one
SELECT *
FROM refunds
WHERE id = $1;


-- name: GetRefundByPaymentID :one
SELECT *
FROM refunds
WHERE payment_id = $1;


-- name: GetRefundByIdempotencyKey :one
SELECT *
FROM refunds
WHERE idempotency_key = $1;


-- name: MarkRefundSucceeded :one
UPDATE refunds
SET
    status = 'SUCCEEDED',
    provider_refund_id = $2,
    updated_at = NOW()
WHERE id = $1
  AND status = 'PENDING'
RETURNING *;


-- name: MarkRefundFailed :one
UPDATE refunds
SET
    status = 'FAILED',
    updated_at = NOW()
WHERE id = $1
  AND status = 'PENDING'
RETURNING *;