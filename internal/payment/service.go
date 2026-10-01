package payment

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

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
	reservationID pgtype.UUID,
	provider string,
	amount int64,
	currency string,
) (db.Payment, error) {
	payment, err := s.queries.CreatePayment(
		ctx,
		db.CreatePaymentParams{
			ReservationID:     reservationID,
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

	_, err = txQueries.ConfirmReservation(
		ctx,
		payment.ReservationID,
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

	payload, err := json.Marshal(map[string]interface{}{
		"payment_id":     payment.ID,
		"reservation_id": payment.ReservationID,
		"amount":         payment.Amount,
		"currency":       payment.Currency,
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
