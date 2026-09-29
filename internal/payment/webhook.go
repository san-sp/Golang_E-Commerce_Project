package payment

import (
	"errors"

	"github.com/jackc/pgx/v5/pgtype"
)

var ErrDuplicateWebhook = errors.New("payment webhook already processed")

type PaymentWebhook struct {
	EventID   string
	EventType string
	PaymentID pgtype.UUID
}
