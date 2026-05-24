// forwarder.go is the second half of the outbox pattern: a background
// loop that drains outbox_messages into the broker.
//
// Relationship to the article: the Three Dots Labs post wires the
// outbox via Watermill's SQL Pub/Sub plus the Watermill Forwarder
// component, which does this job for you out of the box. This file
// shows a hand-rolled equivalent so the mechanism is visible.
// Reach for Watermill's components in real code; read this file to
// understand what they're doing under the hood.
//
// What the forwarder guarantees:
//
//   - At-least-once delivery. A message that has been committed to the
//     outbox will eventually reach the broker, even across forwarder
//     crashes, broker outages, and Postgres failovers.
//
//   - Never publishes a rolled-back message. The forwarder reads only
//     committed rows; uncommitted INSERTs are invisible to it.
//
// What the forwarder explicitly does NOT guarantee:
//
//   - Exactly-once. The window between "broker accepted the message"
//     and "we marked the row published" is real, and on crash we will
//     re-publish. Consumers MUST make their side effects idempotent.
//
//   - Strict global ordering. Within a single forwarder instance the
//     order is insertion-order, but two instances of the forwarder
//     racing will interleave. If you need per-aggregate ordering,
//     partition by aggregate_id and run one forwarder per partition.
package outbox

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/ThreeDotsLabs/watermill"
	"github.com/ThreeDotsLabs/watermill/message"
)

const (
	batchSize     = 100
	pollInterval  = 500 * time.Millisecond
	maxAttempts   = 10           // after this many failures, leave the row for an operator
	backoffOnFail = 5 * time.Second
)

// Forwarder polls the outbox and publishes committed messages.
//
// One Forwarder owns one Publisher. In production you'd run this in
// its own goroutine via a worker pool / supervised process, with a
// liveness check on "rows older than N seconds with published_at IS
// NULL" as your primary alert.
type Forwarder struct {
	db        *sql.DB
	publisher message.Publisher
	logger    watermill.LoggerAdapter
}

func NewForwarder(db *sql.DB, publisher message.Publisher, logger watermill.LoggerAdapter) *Forwarder {
	return &Forwarder{db: db, publisher: publisher, logger: logger}
}

// Run blocks until ctx is cancelled. It polls in a loop; each tick
// drains up to batchSize messages. Polling is simple and fits 99% of
// throughput needs; if you outgrow it, swap to LISTEN/NOTIFY or
// logical replication without changing the rest of the design.
func (f *Forwarder) Run(ctx context.Context) error {
	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			if err := f.drainOnce(ctx); err != nil {
				f.logger.Error("outbox drain failed", err, nil)
				// Don't return — keep polling. Transient DB or broker
				// errors are exactly what this loop exists to ride out.
			}
		}
	}
}

// drainOnce processes one batch.
//
// The lock strategy is the key thing to read carefully:
//
//   - SELECT ... FOR UPDATE SKIP LOCKED grabs only rows no other
//     forwarder is working on. This is what lets you scale by simply
//     running more forwarder replicas — they will not double-publish
//     the same row.
//
//   - The entire publish + mark-as-published cycle happens inside one
//     transaction. If publishing succeeds but committing fails, we
//     will redeliver — that's why consumers must dedup. If publishing
//     fails, we increment attempts and roll back; the row stays
//     visible for the next poll.
func (f *Forwarder) drainOnce(ctx context.Context) error {
	tx, err := f.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin: %w", err)
	}
	defer tx.Rollback() //nolint:errcheck // best-effort; commit path handles success

	rows, err := tx.QueryContext(ctx, `
		SELECT id, topic, payload, metadata, attempts
		FROM outbox_messages
		WHERE published_at IS NULL AND attempts < $1
		ORDER BY created_at
		LIMIT $2
		FOR UPDATE SKIP LOCKED
	`, maxAttempts, batchSize)
	if err != nil {
		return fmt.Errorf("select pending: %w", err)
	}

	type pending struct {
		id       string
		topic    string
		payload  []byte
		metadata []byte
		attempts int
	}
	var batch []pending
	for rows.Next() {
		var p pending
		if err := rows.Scan(&p.id, &p.topic, &p.payload, &p.metadata, &p.attempts); err != nil {
			rows.Close()
			return fmt.Errorf("scan: %w", err)
		}
		batch = append(batch, p)
	}
	rows.Close()
	if len(batch) == 0 {
		return nil
	}

	for _, p := range batch {
		msg := message.NewMessage(p.id, p.payload)

		var metadata map[string]string
		if len(p.metadata) > 0 {
			_ = json.Unmarshal(p.metadata, &metadata)
		}
		for k, v := range metadata {
			msg.Metadata.Set(k, v)
		}

		if pubErr := f.publisher.Publish(p.topic, msg); pubErr != nil {
			// Record the failure and let the next tick retry. Note
			// we update *within* the same transaction; if commit
			// fails too, the attempt counter rolls back, which is
			// actually fine — we'll try again.
			if _, err := tx.ExecContext(ctx, `
				UPDATE outbox_messages
				SET attempts = attempts + 1, last_error = $1
				WHERE id = $2
			`, pubErr.Error(), p.id); err != nil {
				return fmt.Errorf("mark attempt: %w", err)
			}
			f.logger.Error("publish failed", pubErr, watermill.LogFields{
				"id":       p.id,
				"topic":    p.topic,
				"attempts": p.attempts + 1,
			})
			// Brief backoff before the next batch so we don't hammer
			// a broker that's actively unhealthy.
			time.Sleep(backoffOnFail)
			continue
		}

		if _, err := tx.ExecContext(ctx, `
			UPDATE outbox_messages SET published_at = now() WHERE id = $1
		`, p.id); err != nil {
			return fmt.Errorf("mark published: %w", err)
		}
	}

	if err := tx.Commit(); err != nil {
		// On commit failure, the rows we already published will be
		// re-published next time. Consumers dedup; we accept this.
		return fmt.Errorf("commit: %w", err)
	}
	return nil
}

// Sentinel error used by some tests to assert "nothing to do".
var ErrEmpty = errors.New("outbox empty")
