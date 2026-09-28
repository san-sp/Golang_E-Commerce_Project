CREATE TABLE payments (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),

    reservation_id UUID NOT NULL UNIQUE
        REFERENCES reservations(id),

    provider TEXT NOT NULL,

    provider_payment_id TEXT UNIQUE,

    amount BIGINT NOT NULL CHECK (amount >= 0),

    currency TEXT NOT NULL,

    status TEXT NOT NULL
        CHECK (status IN ('PENDING', 'SUCCEEDED', 'FAILED')),

    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);