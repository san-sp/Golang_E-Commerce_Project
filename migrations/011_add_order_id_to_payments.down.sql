DROP INDEX IF EXISTS idx_payments_order_id;

ALTER TABLE payments
DROP CONSTRAINT IF EXISTS payments_order_id_fkey;

ALTER TABLE payments
DROP COLUMN IF EXISTS order_id;