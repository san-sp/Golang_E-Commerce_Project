-- name: GetInventory :one
SELECT
    i.variant_id,
    i.quantity,
    COALESCE(
        SUM(
            CASE
                WHEN r.status = 'ACTIVE'
                 AND r.expires_at > NOW()
                THEN r.quantity
                ELSE 0
            END
        ),
        0
    )::BIGINT AS reserved_quantity,
    (
        i.quantity -
        COALESCE(
            SUM(
                CASE
                    WHEN r.status = 'ACTIVE'
                     AND r.expires_at > NOW()
                    THEN r.quantity
                    ELSE 0
                END
            ),
            0
        )::BIGINT
    )::BIGINT AS available_quantity
FROM inventory i
LEFT JOIN reservations r
    ON r.variant_id = i.variant_id
WHERE i.variant_id = $1
GROUP BY
    i.variant_id,
    i.quantity;

-- name: AdjustInventory :one
UPDATE inventory
SET
    quantity = quantity + $2,
    updated_at = NOW()
WHERE variant_id = $1
  AND quantity + $2 >= 0
RETURNING
    id,
    variant_id,
    quantity,
    created_at,
    updated_at;