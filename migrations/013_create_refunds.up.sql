CREATE TABLE refunds (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),

    payment_id UUID NOT NULL
        REFERENCES payments(id),

    provider_refund_id TEXT UNIQUE,

    amount BIGINT NOT NULL CHECK (amount > 0),

    currency TEXT NOT NULL,

    status TEXT NOT NULL
        CHECK (status IN ('PENDING', 'SUCCEEDED', 'FAILED')),

    idempotency_key TEXT NOT NULL UNIQUE,

    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    UNIQUE (payment_id)
);

CREATE INDEX idx_refunds_status
ON refunds(status);

CREATE INDEX idx_refunds_created_at
ON refunds(created_at);