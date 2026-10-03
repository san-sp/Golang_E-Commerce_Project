-- name: CreatePaymentProcessing :one
INSERT INTO payment_processing (
    payment_id
)
VALUES ($1)
RETURNING
    id,
    payment_id,
    processed_at;

-- name: GetPaymentProcessing :one
SELECT
    id,
    payment_id,
    processed_at
FROM payment_processing
WHERE payment_id = $1;