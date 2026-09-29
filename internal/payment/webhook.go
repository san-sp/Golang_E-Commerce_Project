package payment

import (
	"errors"

	"github.com/jackc/pgx/v5/pgtype"
)

var ErrDuplicateWebhook = errors.New("payment webhook already processed")

type PaymentWebhook struct {
	EventID   string      `json:"event_id"`
	EventType string      `json:"event_type"`
	PaymentID pgtype.UUID `json:"payment_id"`
}
