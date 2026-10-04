ALTER TABLE payments
ALTER COLUMN reservation_id DROP NOT NULL;

ALTER TABLE payments
DROP CONSTRAINT IF EXISTS payments_reservation_id_key;