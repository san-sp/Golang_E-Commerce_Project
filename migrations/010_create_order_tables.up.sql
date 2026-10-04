CREATE TABLE orders (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),

    status TEXT NOT NULL DEFAULT 'PENDING'
        CHECK (
            status IN (
                'PENDING',
                'CONFIRMED',
                'PROCESSING',
                'SHIPPED',
                'DELIVERED',
                'CANCELLED',
                'REFUNDED'
            )
        ),

    total_amount BIGINT NOT NULL CHECK (total_amount >= 0),

    currency TEXT NOT NULL,

    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE order_items (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),

    order_id UUID NOT NULL
        REFERENCES orders(id)
        ON DELETE CASCADE,

    variant_id UUID NOT NULL
        REFERENCES product_variants(id),

    quantity BIGINT NOT NULL
        CHECK (quantity > 0),

    unit_price BIGINT NOT NULL
        CHECK (unit_price >= 0),

    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_order_items_order_id
    ON order_items(order_id);

CREATE INDEX idx_order_items_variant_id
    ON order_items(variant_id);

CREATE INDEX idx_orders_status
    ON orders(status);

CREATE INDEX idx_orders_created_at
    ON orders(created_at);