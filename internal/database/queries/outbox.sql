-- name: ListPendingOutboxEvents :many
SELECT
    id,
    event_type,
    payload,
    created_at,
    published_at,
    last_attempted_at,
    attempts,
    status,
    processing_at
FROM outbox_events
WHERE status = 'PENDING'
ORDER BY created_at;

-- name: ClaimPendingOutboxEvents :many
WITH claimed AS (
    SELECT id
    FROM outbox_events
    WHERE status = 'PENDING'
    ORDER BY created_at
    LIMIT 100
    FOR UPDATE SKIP LOCKED
)
UPDATE outbox_events AS o
SET
    status = 'PROCESSING',
    processing_at = NOW()
FROM claimed
WHERE o.id = claimed.id
RETURNING
    o.id,
    o.event_type,
    o.payload,
    o.created_at,
    o.published_at,
    o.last_attempted_at,
    o.attempts,
    o.status,
    o.processing_at;

-- name: MarkOutboxPublishAttempt :exec
UPDATE outbox_events
SET
    attempts = attempts + 1,
    last_attempted_at = NOW()
WHERE id = $1
  AND status = 'PROCESSING';

-- name: MarkOutboxEventPublished :exec
UPDATE outbox_events
SET
    status = 'PUBLISHED',
    published_at = NOW()
WHERE id = $1
  AND status = 'PROCESSING';    