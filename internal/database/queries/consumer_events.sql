-- name: RecordConsumerEvent :one
INSERT INTO consumer_events (
    consumer_name,
    event_id
)
VALUES (
    $1,
    $2
)
ON CONFLICT (consumer_name, event_id) DO NOTHING
RETURNING
    id,
    consumer_name,
    event_id,
    processed_at;