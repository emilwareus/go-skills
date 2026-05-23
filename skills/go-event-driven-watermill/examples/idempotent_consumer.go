// idempotent_consumer.go is a supplementary example: the Three Dots
// Labs article on distributed transactions calls out that consumers
// must dedup ("duplicate delivery is expected") but does not show
// the dedup implementation in code. This file fills that gap using
// the article's OnPointsUsedForDiscountHandler as the consumer.
//
// Why dedup is mandatory:
//
//   - The outbox forwarder gives at-least-once delivery. The window
//     between "broker accepted the message" and "we marked the
//     outbox row published" is real; a crash there causes
//     redelivery.
//   - Brokers themselves redeliver on consumer crashes, rebalances,
//     and timeouts.
//   - Therefore any consumer that performs a side effect (calling
//     AddDiscountHandler.Handle, which mutates user_discounts) MUST
//     be safe against seeing the same message twice.
//
// Two strategies are shown:
//
//  1. processed_messages table — the general-purpose answer.
//  2. Natural idempotency via a unique constraint — preferred when
//     the side effect already has a natural unique key.
package examples

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
)

// Schema:
//
//	CREATE TABLE processed_messages (
//	    handler_name TEXT NOT NULL,
//	    message_id   TEXT NOT NULL,
//	    processed_at TIMESTAMPTZ NOT NULL DEFAULT now(),
//	    PRIMARY KEY (handler_name, message_id)
//	);
//
// The composite key matters: the same PointsUsedForDiscount event
// may be consumed by several handlers (orders service, analytics,
// notifications). Each handler needs its own dedup record.
type ProcessedMessages struct {
	db          *sql.DB
	handlerName string
}

func NewProcessedMessages(db *sql.DB, handlerName string) *ProcessedMessages {
	return &ProcessedMessages{db: db, handlerName: handlerName}
}

// MarkOrSkip returns true if this message is new and the caller
// should run the handler, or false if it was already processed
// (the caller should ACK without doing anything).
//
// The critical detail: this call MUST be inside the same DB
// transaction as the handler's side effect. If we marked first and
// then crashed before doing the side effect, the work would be
// dropped permanently. See IdempotentOnPointsUsedForDiscount below
// for the wiring.
func (p *ProcessedMessages) MarkOrSkip(ctx context.Context, tx *sql.Tx, messageID string) (proceed bool, err error) {
	res, err := tx.ExecContext(ctx, `
		INSERT INTO processed_messages (handler_name, message_id)
		VALUES ($1, $2)
		ON CONFLICT DO NOTHING
	`, p.handlerName, messageID)
	if err != nil {
		return false, fmt.Errorf("mark processed: %w", err)
	}
	rows, err := res.RowsAffected()
	if err != nil {
		return false, fmt.Errorf("rows affected: %w", err)
	}
	return rows == 1, nil
}

// IdempotentOnPointsUsedForDiscount wraps the article's
// OnPointsUsedForDiscountHandler with the dedup check. The inner
// handler must accept a *sql.Tx and do its side effect inside that
// tx — that constraint is what makes the dedup crash-safe.
type IdempotentOnPointsUsedForDiscount struct {
	db        *sql.DB
	processed *ProcessedMessages
	handler   func(ctx context.Context, tx *sql.Tx, event *PointsUsedForDiscount) error
}

func NewIdempotentOnPointsUsedForDiscount(
	db *sql.DB,
	processed *ProcessedMessages,
	handler func(ctx context.Context, tx *sql.Tx, event *PointsUsedForDiscount) error,
) *IdempotentOnPointsUsedForDiscount {
	return &IdempotentOnPointsUsedForDiscount{db: db, processed: processed, handler: handler}
}

// Handle is the Watermill-facing entry point. Returning nil ACKs;
// returning an error NACKs and triggers redelivery (which we are
// safe against because of MarkOrSkip).
//
// The argument list here would normally be a *message.Message; we
// take messageID + event explicitly to keep the example focused on
// the pattern rather than on Watermill's marshalling.
func (h *IdempotentOnPointsUsedForDiscount) Handle(ctx context.Context, messageID string, event *PointsUsedForDiscount) (err error) {
	tx, err := h.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin: %w", err)
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback()
		}
	}()

	proceed, err := h.processed.MarkOrSkip(ctx, tx, messageID)
	if err != nil {
		return err
	}
	if !proceed {
		// Already processed in a previous delivery. ACK and move on.
		return tx.Commit()
	}

	if err = h.handler(ctx, tx, event); err != nil {
		return err
	}
	return tx.Commit()
}

// ── Alternate: natural idempotency via unique constraint ───────────
//
// If your handler's side effect is "insert one row per event", a
// unique constraint dedups for free. Reach for this before reaching
// for processed_messages — it's simpler and one fewer moving part.
//
// For PointsUsedForDiscount the orders service might write:
//
//	CREATE TABLE applied_discounts (
//	    event_id   TEXT PRIMARY KEY,           -- the broker message UUID
//	    user_id    INT  NOT NULL,
//	    discount   INT  NOT NULL,
//	    applied_at TIMESTAMPTZ DEFAULT now()
//	);
//
//	_, err := tx.ExecContext(ctx, `
//	    INSERT INTO applied_discounts (event_id, user_id, discount)
//	    VALUES ($1, $2, $3)
//	`, messageID, event.UserID, event.Points)
//	if isUniqueViolation(err) {
//	    return nil // already applied; ACK
//	}
//	if err != nil { return err }
//	return updateUserDiscount(ctx, tx, event.UserID, event.Points)
//
// The unique constraint is the dedup. No second table required.
var ErrAlreadyProcessed = errors.New("message already processed")

// PointsUsedForDiscount is duplicated here so the file reads
// standalone. In a real codebase the events package would be shared
// between publisher and consumer.
type PointsUsedForDiscount struct {
	UserID int `json:"user_id"`
	Points int `json:"points"`
}
