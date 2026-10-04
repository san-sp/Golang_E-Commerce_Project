package payment

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/san-sp/Golang_E-Commerce_Project/internal/database/db"
)

type Service struct {
	pool     *pgxpool.Pool
	queries  *db.Queries
	provider PaymentProvider
}

func NewService(
	pool *pgxpool.Pool,
	queries *db.Queries,
	provider PaymentProvider,
) *Service {
	return &Service{
		pool:     pool,
		queries:  queries,
		provider: provider,
	}
}

func (s *Service) CreatePayment(
	ctx context.Context,
	orderID pgtype.UUID,
	provider string,
	amount int64,
	currency string,
) (db.Payment, error) {
	payment, err := s.queries.CreatePayment(
		ctx,
		db.CreatePaymentParams{
			OrderID:           orderID,
			Provider:          provider,
			ProviderPaymentID: pgtype.Text{},
			Amount:            amount,
			Currency:          currency,
			Status:            "PENDING",
		},
	)
	if err != nil {
		return db.Payment{}, fmt.Errorf("create payment: %w", err)
	}

	providerPaymentID, err := s.provider.CreatePayment(
		ctx,
		amount,
		currency,
	)
	if err != nil {
		if errors.Is(err, ErrProviderFailed) {
			payment, updateErr := s.queries.MarkPaymentFailed(
				ctx,
				payment.ID,
			)
			if updateErr != nil {
				return db.Payment{}, fmt.Errorf(
					"mark payment failed: %w",
					updateErr,
				)
			}

			return payment, fmt.Errorf(
				"create provider payment: %w",
				err,
			)
		}

		if errors.Is(err, ErrProviderUnknown) {
			return payment, fmt.Errorf(
				"provider payment outcome unknown: %w",
				err,
			)
		}

		return db.Payment{}, fmt.Errorf(
			"create provider payment: %w",
			err,
		)
	}

	providerPayment := pgtype.Text{
		String: providerPaymentID,
		Valid:  true,
	}

	payment, err = s.queries.UpdatePaymentProviderID(
		ctx,
		db.UpdatePaymentProviderIDParams{
			ID:                payment.ID,
			ProviderPaymentID: providerPayment,
		},
	)
	if err != nil {
		return db.Payment{}, fmt.Errorf("update provider payment ID: %w", err)
	}

	return payment, nil
}

func (s *Service) MarkPaymentSucceeded(
	ctx context.Context,
	paymentID pgtype.UUID,
) (db.Payment, error) {
	payment, err := s.queries.MarkPaymentSucceeded(ctx, paymentID)
	if err != nil {
		return db.Payment{}, fmt.Errorf(
			"mark payment succeeded: %w",
			err,
		)
	}

	return payment, nil
}

func (s *Service) ProcessPaymentWebhook(
	ctx context.Context,
	webhook PaymentWebhook,
) (db.Payment, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return db.Payment{}, fmt.Errorf(
			"begin payment webhook transaction: %w",
			err,
		)
	}
	defer tx.Rollback(ctx)

	txQueries := s.queries.WithTx(tx)

	_, err = txQueries.RecordPaymentEvent(
		ctx,
		db.RecordPaymentEventParams{
			EventID:   webhook.EventID,
			PaymentID: webhook.PaymentID,
			EventType: webhook.EventType,
		},
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return db.Payment{}, ErrDuplicateWebhook
		}

		return db.Payment{}, fmt.Errorf(
			"record payment event: %w",
			err,
		)
	}

	payment, err := txQueries.MarkPaymentSucceeded(
		ctx,
		webhook.PaymentID,
	)
	if err != nil {
		return db.Payment{}, fmt.Errorf(
			"mark payment succeeded: %w",
			err,
		)
	}

	reservations, err := txQueries.GetReservationsByOrderID(
		ctx,
		payment.OrderID,
	)
	if err != nil {
		return db.Payment{}, fmt.Errorf(
			"get order reservations: %w",
			err,
		)
	}

	if len(reservations) == 0 {
		return db.Payment{}, fmt.Errorf(
			"no reservations found for order",
		)
	}

	now := time.Now()

	for _, reservation := range reservations {
		if reservation.Status != "ACTIVE" ||
			!reservation.ExpiresAt.Valid ||
			!reservation.ExpiresAt.Time.After(now) {
			return db.Payment{}, ErrReservationExpired
		}
	}

	for _, reservation := range reservations {
		_, err = txQueries.ConfirmReservation(
			ctx,
			reservation.ID,
		)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return db.Payment{}, ErrReservationExpired
			}

			return db.Payment{}, fmt.Errorf(
				"confirm reservation: %w",
				err,
			)
		}
	}

	_, err = txQueries.ConfirmOrder(
		ctx,
		payment.OrderID,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return db.Payment{}, fmt.Errorf(
				"confirm order: %w",
				err,
			)
		}

		return db.Payment{}, fmt.Errorf(
			"confirm order: %w",
			err,
		)
	}

	payload, err := json.Marshal(map[string]interface{}{
		"payment_id": payment.ID,
		"order_id":   payment.OrderID,
		"amount":     payment.Amount,
		"currency":   payment.Currency,
	})
	if err != nil {
		return db.Payment{}, fmt.Errorf(
			"marshal payment succeeded event: %w",
			err,
		)
	}

	_, err = txQueries.CreateOutboxEvent(
		ctx,
		db.CreateOutboxEventParams{
			EventType: "PAYMENT_SUCCEEDED",
			Payload:   payload,
		},
	)
	if err != nil {
		return db.Payment{}, fmt.Errorf(
			"create payment succeeded outbox event: %w",
			err,
		)
	}

	if err := tx.Commit(ctx); err != nil {
		return db.Payment{}, fmt.Errorf(
			"commit payment webhook transaction: %w",
			err,
		)
	}

	return payment, nil
}
