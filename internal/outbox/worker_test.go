package outbox

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/joho/godotenv"

	"github.com/san-sp/Golang_E-Commerce_Project/internal/database"
	"github.com/san-sp/Golang_E-Commerce_Project/internal/database/db"
)

func TestOutboxClaimOwnership(t *testing.T) {
	if err := godotenv.Load("../../.env"); err != nil {
		t.Fatalf("load .env: %v", err)
	}

	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		t.Fatal("DATABASE_URL is not set")
	}

	ctx := context.Background()

	pool, err := database.NewPostgres(ctx, databaseURL)
	if err != nil {
		t.Fatalf("connect to PostgreSQL: %v", err)
	}
	t.Cleanup(func() {
		pool.Close()
	})

	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatalf("begin test transaction: %v", err)
	}
	defer func() {
		_ = tx.Rollback(context.Background())
	}()

	queries := db.New(tx)

	// Create an isolated event for this test.
	event, err := queries.CreateOutboxEvent(
		ctx,
		db.CreateOutboxEventParams{
			EventType: "ORDER_CREATED",
			Payload:   []byte(`{"test":true}`),
		},
	)
	if err != nil {
		t.Fatalf("create test outbox event: %v", err)
	}

	t.Cleanup(func() {
		_, cleanupErr := pool.Exec(
			context.Background(),
			`DELETE FROM outbox_events WHERE id = $1`,
			event.ID,
		)
		if cleanupErr != nil {
			t.Errorf("cleanup test outbox event: %v", cleanupErr)
		}
	})

	_, err = tx.Exec(
		ctx,
		`UPDATE outbox_events
     SET status = 'PROCESSING',
         processing_at = NOW(),
         processing_token = NULL
     WHERE id <> $1
       AND status = 'PENDING'`,
		event.ID,
	)
	if err != nil {
		t.Fatalf("isolate pending events: %v", err)
	}

	_, err = tx.Exec(
		ctx,
		`UPDATE outbox_events
     SET processing_at = NOW()
     WHERE id <> $1
       AND status = 'PROCESSING'`,
		event.ID,
	)
	if err != nil {
		t.Fatalf("refresh other processing timestamps: %v", err)
	}

	_, err = tx.Exec(
		ctx,
		`UPDATE outbox_events
     SET status = 'PENDING',
         processing_at = NULL,
         processing_token = NULL
     WHERE id = $1`,
		event.ID,
	)
	if err != nil {
		t.Fatalf("prepare test event: %v", err)
	}

	// Worker A claims the event.
	claimedA, err := queries.ClaimPendingOutboxEvents(ctx, 10)
	if err != nil {
		t.Fatalf("worker A claim: %v", err)
	}

	var tokenA pgtype.UUID
	found := false
	for _, claimed := range claimedA {
		if claimed.ID == event.ID {
			tokenA = claimed.ProcessingToken
			found = true
			break
		}
	}
	if !found {
		t.Fatal("worker A did not claim the test event")
	}
	if !tokenA.Valid {
		t.Fatal("worker A received an invalid processing token")
	}

	// Simulate a stale claim without waiting five minutes.
	_, err = tx.Exec(
		ctx,
		`UPDATE outbox_events
     SET processing_at = $2
     WHERE id = $1`,
		event.ID,
		time.Now().Add(-6*time.Minute),
	)
	if err != nil {
		t.Fatalf("make claim stale: %v", err)
	}

	if err := queries.RecoverStaleOutboxEvents(ctx); err != nil {
		t.Fatalf("recover stale event: %v", err)
	}

	var status string
	var processingAt *time.Time
	var processingToken pgtype.UUID

	err = tx.QueryRow(
		ctx,
		`SELECT status, processing_at, processing_token
     FROM outbox_events
     WHERE id = $1`,
		event.ID,
	).Scan(&status, &processingAt, &processingToken)
	if err != nil {
		t.Fatalf("inspect event after recovery: %v", err)
	}

	if status != "PENDING" {
		t.Fatalf("status after recovery = %q, want PENDING", status)
	}

	if processingAt != nil {
		t.Fatalf("processing_at after recovery = %v, want NULL", *processingAt)
	}

	if processingToken.Valid {
		t.Fatal("processing_token after recovery is still valid; want NULL")
	}

	// Worker B reclaims the same event with a fresh token.
	claimedB, err := queries.ClaimPendingOutboxEvents(ctx, 10)
	if err != nil {
		t.Fatalf("worker B claim: %v", err)
	}

	var tokenB pgtype.UUID
	found = false
	for _, claimed := range claimedB {
		if claimed.ID == event.ID {
			tokenB = claimed.ProcessingToken
			found = true
			break
		}
	}
	if !found {
		t.Fatal("worker B did not reclaim the test event")
	}
	if !tokenB.Valid {
		t.Fatal("worker B received an invalid processing token")
	}
	if tokenA.Bytes == tokenB.Bytes {
		t.Fatal("reclaimed event has the same token as the previous claim")
	}

	// Worker A must not mark an event published using its stale token.
	rows, err := queries.MarkOutboxEventPublished(
		ctx,
		db.MarkOutboxEventPublishedParams{
			ID:              event.ID,
			ProcessingToken: tokenA,
		},
	)
	if err != nil {
		t.Fatalf("mark published with stale token: %v", err)
	}
	if rows != 0 {
		t.Fatalf("stale token updated %d rows; want 0", rows)
	}

	// Worker B still owns the claim and can mark the event published.
	rows, err = queries.MarkOutboxEventPublished(
		ctx,
		db.MarkOutboxEventPublishedParams{
			ID:              event.ID,
			ProcessingToken: tokenB,
		},
	)
	if err != nil {
		t.Fatalf("mark published with current token: %v", err)
	}
	if rows != 1 {
		t.Fatalf("current token updated %d rows; want 1", rows)
	}
}
