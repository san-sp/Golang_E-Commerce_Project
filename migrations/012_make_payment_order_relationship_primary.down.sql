ALTER TABLE payments
ADD CONSTRAINT payments_reservation_id_key
UNIQUE (reservation_id);

ALTER TABLE payments
ALTER COLUMN reservation_id SET NOT NULL;