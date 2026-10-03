CREATE TABLE payment_processing (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    payment_id UUID NOT NULL REFERENCES payments(id),
    processed_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    UNIQUE (payment_id)
);