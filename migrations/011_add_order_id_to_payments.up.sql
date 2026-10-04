ALTER TABLE payments
ADD COLUMN order_id UUID;

ALTER TABLE payments
ADD CONSTRAINT payments_order_id_fkey
FOREIGN KEY (order_id)
REFERENCES orders(id);

CREATE UNIQUE INDEX idx_payments_order_id
ON payments(order_id);