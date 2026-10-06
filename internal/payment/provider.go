package payment

import (
	"context"
	"errors"
)

var (
	ErrProviderFailed  = errors.New("payment provider failed")
	ErrProviderUnknown = errors.New("payment provider outcome unknown")
)

type PaymentProvider interface {
	CreatePayment(
		ctx context.Context,
		amount int64,
		currency string,
	) (string, error)

	RefundPayment(
		ctx context.Context,
		providerPaymentID string,
		amount int64,
		currency string,
		idempotencyKey string,
	) (string, error)
}
