package payment

import "github.com/jackc/pgx/v5/pgtype"

type PaymentSucceededEvent struct {
	PaymentID pgtype.UUID `json:"payment_id"`
	OrderID   pgtype.UUID `json:"order_id"`
	Amount    int64       `json:"amount"`
	Currency  string      `json:"currency"`
}
