package payment

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/san-sp/Golang_E-Commerce_Project/internal/database/db"
)

var (
	ErrRefundNotFound       = errors.New("refund not found")
	ErrPaymentNotFound      = errors.New("payment not found")
	ErrPaymentNotRefundable = errors.New("payment is not refundable")
	ErrOrderNotRefundable   = errors.New("order is not refundable")
	ErrRefundAlreadyExists  = errors.New("refund already exists")
	ErrInvalidIdempotency   = errors.New("invalid idempotency key")
)

type RefundService struct {
	pool     *pgxpool.Pool
	queries  *db.Queries
	provider PaymentProvider
}

func NewRefundService(
	pool *pgxpool.Pool,
	queries *db.Queries,
	provider PaymentProvider,
) *RefundService {
	return &RefundService{
		pool:     pool,
		queries:  queries,
		provider: provider,
	}
}

func (s *RefundService) CreateRefund(
	ctx context.Context,
	paymentID pgtype.UUID,
	idempotencyKey string,
) (db.Refund, error) {
	if !paymentID.Valid {
		return db.Refund{}, ErrPaymentNotFound
	}

	if idempotencyKey == "" {
		return db.Refund{}, ErrInvalidIdempotency
	}

	// If this exact idempotency key was already used,
	// return the existing refund.
	existing, err := s.queries.GetRefundByIdempotencyKey(
		ctx,
		idempotencyKey,
	)
	if err == nil {
		return existing, nil
	}

	if !errors.Is(err, pgx.ErrNoRows) {
		return db.Refund{}, fmt.Errorf(
			"get refund by idempotency key: %w",
			err,
		)
	}

	// Load payment from our database.
	payment, err := s.queries.GetPayment(ctx, paymentID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return db.Refund{}, ErrPaymentNotFound
		}

		return db.Refund{}, fmt.Errorf(
			"get payment: %w",
			err,
		)
	}

	// Only a successful payment can be refunded.
	if payment.Status != "SUCCEEDED" {
		return db.Refund{}, ErrPaymentNotRefundable
	}

	// We need the provider payment ID for the external refund.
	if !payment.ProviderPaymentID.Valid ||
		payment.ProviderPaymentID.String == "" {
		return db.Refund{}, fmt.Errorf(
			"payment has no provider payment ID",
		)
	}

	if !payment.OrderID.Valid {
		return db.Refund{}, fmt.Errorf(
			"payment has no order ID",
		)
	}

	// The order must still be CONFIRMED.
	order, err := s.queries.GetOrder(ctx, payment.OrderID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return db.Refund{}, ErrOrderNotRefundable
		}

		return db.Refund{}, fmt.Errorf(
			"get order: %w",
			err,
		)
	}

	if order.Status != "CONFIRMED" {
		return db.Refund{}, ErrOrderNotRefundable
	}

	// Create our refund intent before calling the provider.
	refund, err := s.queries.CreateRefund(
		ctx,
		db.CreateRefundParams{
			PaymentID:      payment.ID,
			Amount:         payment.Amount,
			Currency:       payment.Currency,
			IdempotencyKey: idempotencyKey,
		},
	)
	if err != nil {
		// Another request may have won the race using the same
		// idempotency key or the same payment.
		if isUniqueViolation(err) {
			existing, lookupErr := s.queries.GetRefundByIdempotencyKey(
				ctx,
				idempotencyKey,
			)
			if lookupErr == nil {
				return existing, nil
			}

			if !errors.Is(lookupErr, pgx.ErrNoRows) {
				return db.Refund{}, fmt.Errorf(
					"get existing refund after duplicate: %w",
					lookupErr,
				)
			}

			return db.Refund{}, ErrRefundAlreadyExists
		}

		return db.Refund{}, fmt.Errorf(
			"create refund: %w",
			err,
		)
	}

	// IMPORTANT:
	// Never hold a database transaction while calling
	// an external payment provider.
	providerRefundID, err := s.provider.RefundPayment(
		ctx,
		payment.ProviderPaymentID.String,
		payment.Amount,
		payment.Currency,
		idempotencyKey,
	)
	if err != nil {
		if errors.Is(err, ErrProviderFailed) {
			failedRefund, updateErr := s.queries.MarkRefundFailed(
				ctx,
				refund.ID,
			)
			if updateErr != nil {
				return db.Refund{}, fmt.Errorf(
					"mark refund failed: %w",
					updateErr,
				)
			}

			return failedRefund, fmt.Errorf(
				"refund provider failed: %w",
				err,
			)
		}

		// The provider may have processed the refund even though
		// we don't know the result. Leave the refund PENDING.
		if errors.Is(err, ErrProviderUnknown) {
			return refund, fmt.Errorf(
				"refund provider outcome unknown: %w",
				err,
			)
		}

		return db.Refund{}, fmt.Errorf(
			"refund provider: %w",
			err,
		)
	}

	return s.finalizeRefund(
		ctx,
		refund.ID,
		providerRefundID,
	)
}

func (s *RefundService) finalizeRefund(
	ctx context.Context,
	refundID pgtype.UUID,
	providerRefundID string,
) (db.Refund, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return db.Refund{}, fmt.Errorf(
			"begin refund finalization transaction: %w",
			err,
		)
	}
	defer tx.Rollback(ctx)

	txQueries := s.queries.WithTx(tx)

	// PENDING → SUCCEEDED.
	//
	// This conditional transition is what prevents a successful
	// refund from restoring inventory more than once.
	refund, err := txQueries.MarkRefundSucceeded(
		ctx,
		db.MarkRefundSucceededParams{
			ID: refundID,
			ProviderRefundID: pgtype.Text{
				String: providerRefundID,
				Valid:  true,
			},
		},
	)
	if err != nil {
		return db.Refund{}, fmt.Errorf(
			"mark refund succeeded: %w",
			err,
		)
	}

	payment, err := txQueries.GetPayment(
		ctx,
		refund.PaymentID,
	)
	if err != nil {
		return db.Refund{}, fmt.Errorf(
			"get refund payment: %w",
			err,
		)
	}

	if !payment.OrderID.Valid {
		return db.Refund{}, fmt.Errorf(
			"payment has no order ID",
		)
	}

	reservations, err := txQueries.GetReservationsByOrderID(
		ctx,
		payment.OrderID,
	)
	if err != nil {
		return db.Refund{}, fmt.Errorf(
			"get order reservations: %w",
			err,
		)
	}

	if len(reservations) == 0 {
		return db.Refund{}, fmt.Errorf(
			"no reservations found for order",
		)
	}

	// Every reservation belonging to the paid order must be
	// CONFIRMED before we restore its inventory.
	for _, reservation := range reservations {
		if reservation.Status != "CONFIRMED" {
			return db.Refund{}, fmt.Errorf(
				"reservation %s is not confirmed",
				reservation.ID,
			)
		}

		_, err = txQueries.RestoreInventory(
			ctx,
			db.RestoreInventoryParams{
				VariantID: reservation.VariantID,
				Quantity:  reservation.Quantity,
			},
		)
		if err != nil {
			return db.Refund{}, fmt.Errorf(
				"restore inventory: %w",
				err,
			)
		}
	}

	// CONFIRMED → REFUNDED.
	_, err = txQueries.RefundOrder(
		ctx,
		payment.OrderID,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return db.Refund{}, ErrOrderNotRefundable
		}

		return db.Refund{}, fmt.Errorf(
			"refund order: %w",
			err,
		)
	}

	payload, err := json.Marshal(map[string]interface{}{
		"refund_id":  refund.ID,
		"payment_id": refund.PaymentID,
		"order_id":   payment.OrderID,
		"amount":     refund.Amount,
		"currency":   refund.Currency,
	})
	if err != nil {
		return db.Refund{}, fmt.Errorf(
			"marshal payment refunded event: %w",
			err,
		)
	}

	_, err = txQueries.CreateOutboxEvent(
		ctx,
		db.CreateOutboxEventParams{
			EventType: "PAYMENT_REFUNDED",
			Payload:   payload,
		},
	)
	if err != nil {
		return db.Refund{}, fmt.Errorf(
			"create payment refunded outbox event: %w",
			err,
		)
	}

	if err := tx.Commit(ctx); err != nil {
		return db.Refund{}, fmt.Errorf(
			"commit refund finalization transaction: %w",
			err,
		)
	}

	return refund, nil
}

func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError

	return errors.As(err, &pgErr) &&
		pgErr.Code == "23505"
}
