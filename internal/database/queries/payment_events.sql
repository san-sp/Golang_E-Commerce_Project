-- name: RecordPaymentEvent :one
INSERT INTO payment_events (
    event_id,
    payment_id,
    event_type
)
VALUES ($1, $2, $3)
ON CONFLICT (event_id) DO NOTHING
RETURNING id, event_id, payment_id, event_type, created_at;