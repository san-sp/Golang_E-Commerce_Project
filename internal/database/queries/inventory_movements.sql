-- name: CreateInventoryMovement :one
INSERT INTO inventory_movements (
    variant_id,
    movement_type,
    quantity,
    reference_type,
    reference_id
)
VALUES ($1, $2, $3, $4, $5)
RETURNING
    id,
    variant_id,
    movement_type,
    quantity,
    reference_type,
    reference_id,
    created_at;

-- name: ListInventoryMovements :many
SELECT
    id,
    variant_id,
    movement_type,
    quantity,
    reference_type,
    reference_id,
    created_at
FROM inventory_movements
WHERE variant_id = $1
ORDER BY created_at DESC;