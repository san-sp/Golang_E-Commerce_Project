CREATE TABLE inventory_movements (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),

    variant_id UUID NOT NULL
        REFERENCES product_variants(id),

    movement_type TEXT NOT NULL,

    quantity BIGINT NOT NULL
        CHECK (quantity <> 0),

    reference_type TEXT,

    reference_id UUID,

    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_inventory_movements_variant_id_created_at
ON inventory_movements (variant_id, created_at DESC);