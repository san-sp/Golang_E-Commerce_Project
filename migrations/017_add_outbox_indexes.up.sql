CREATE INDEX idx_outbox_pending_created_at
ON outbox_events (created_at)
WHERE status = 'PENDING';

CREATE INDEX idx_outbox_published_at
ON outbox_events (published_at)
WHERE status = 'PUBLISHED';
