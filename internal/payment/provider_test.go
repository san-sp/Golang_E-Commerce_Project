package payment

import (
	"context"
	"errors"
	"testing"
)

func TestMockProviderImplementsPaymentProvider(t *testing.T) {
	var provider PaymentProvider = NewMockProvider()

	paymentID, err := provider.CreatePayment(
		context.Background(),
		899900,
		"INR",
	)
	if err != nil {
		t.Fatalf("create payment: %v", err)
	}

	if paymentID == "" {
		t.Fatal("expected provider payment ID")
	}

	t.Logf("mock provider payment ID: %s", paymentID)
}

func TestMockProviderFailure(t *testing.T) {
	provider := NewMockProvider()
	provider.SetFailure(true)

	_, err := provider.CreatePayment(
		context.Background(),
		899900,
		"INR",
	)

	if !errors.Is(err, ErrProviderFailed) {
		t.Fatalf(
			"expected ErrProviderFailed, got %v",
			err,
		)
	}
}

func TestMockProviderUnknown(t *testing.T) {
	provider := NewMockProvider()
	provider.SetUnknown()

	_, err := provider.CreatePayment(
		context.Background(),
		899900,
		"INR",
	)

	if !errors.Is(err, ErrProviderUnknown) {
		t.Fatalf(
			"expected ErrProviderUnknown, got %v",
			err,
		)
	}
}
