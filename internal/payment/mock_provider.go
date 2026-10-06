package payment

import (
	"context"

	"github.com/google/uuid"
)

type MockProvider struct {
	failureMode string
}

func NewMockProvider() *MockProvider {
	return &MockProvider{}
}

func (m *MockProvider) SetFailure(shouldFail bool) {
	if shouldFail {
		m.failureMode = "failed"
	} else {
		m.failureMode = ""
	}
}

func (m *MockProvider) SetUnknown() {
	m.failureMode = "unknown"
}

func (m *MockProvider) CreatePayment(
	ctx context.Context,
	amount int64,
	currency string,
) (string, error) {
	switch m.failureMode {
	case "failed":
		return "", ErrProviderFailed
	case "unknown":
		return "", ErrProviderUnknown
	}

	return "mock_payment_" + uuid.NewString(), nil
}

func (m *MockProvider) RefundPayment(
	ctx context.Context,
	providerPaymentID string,
	amount int64,
	currency string,
	idempotencyKey string,
) (string, error) {
	switch m.failureMode {
	case "failed":
		return "", ErrProviderFailed

	case "unknown":
		return "", ErrProviderUnknown
	}

	return "mock_refund_" + uuid.NewString(), nil
}
