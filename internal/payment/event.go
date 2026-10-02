package payment

import "github.com/jackc/pgx/v5/pgtype"

type PaymentSucceededEvent struct {
	PaymentID     pgtype.UUID `json:"payment_id"`
	ReservationID pgtype.UUID `json:"reservation_id"`
	Amount        int64       `json:"amount"`
	Currency      string      `json:"currency"`
}
