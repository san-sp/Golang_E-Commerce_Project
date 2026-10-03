package payment

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/san-sp/Golang_E-Commerce_Project/internal/database/db"
)

var ErrConsumerEventAlreadyProcessed = errors.New(
	"consumer event already processed",
)

type ConsumerService struct {
	pool    *pgxpool.Pool
	queries *db.Queries
}

func NewConsumerService(
	pool *pgxpool.Pool,
	queries *db.Queries,
) *ConsumerService {
	return &ConsumerService{
		pool:    pool,
		queries: queries,
	}
}

func (s *ConsumerService) ProcessPaymentSucceeded(
	ctx context.Context,
	consumerName string,
	eventID pgtype.UUID,
	paymentID pgtype.UUID,
) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin consumer transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	txQueries := s.queries.WithTx(tx)

	_, err = txQueries.RecordConsumerEvent(
		ctx,
		db.RecordConsumerEventParams{
			ConsumerName: consumerName,
			EventID:      eventID,
		},
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrConsumerEventAlreadyProcessed
		}

		return fmt.Errorf("record consumer event: %w", err)
	}

	_, err = txQueries.CreatePaymentProcessing(
		ctx,
		paymentID,
	)
	if err != nil {
		return fmt.Errorf(
			"create payment processing: %w",
			err,
		)
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf(
			"commit consumer transaction: %w",
			err,
		)
	}

	return nil
}
