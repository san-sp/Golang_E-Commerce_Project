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