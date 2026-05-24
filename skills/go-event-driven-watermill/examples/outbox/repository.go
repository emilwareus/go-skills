// Package outbox shows the repository side of the article's
// version-3 (outbox) UsePointsAsDiscount flow. The handler — which
// supplies a closure returning ([]any) of events — lives in the
// parent directory (handler_returns_events.go).
//
// What this file demonstrates:
//
//   - UpdateByID with the enriched signature
//     func(user *User) (bool, []any, error).
//
//   - The repository inserts each returned event into the outbox
//     table inside the SAME transaction as the users/user_discounts
//     UPDATEs. The article uses Watermill's SQL Pub/Sub for this
//     (see "Repository Publishing Logic" snippet from the post);
//     this file shows the underlying mechanism with plain INSERTs
//     so the pattern is visible without depending on Watermill's
//     internal table layout.
//
// Reading order: parent's handler_returns_events.go → this file →
// forwarder.go.
package outbox

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"reflect"

	"github.com/google/uuid"
)

// User / Discounts mirror the article's aggregate. Duplicated in this
// example folder so each file can be read standalone; in a real
// project they would come from one domain package.
type User struct {
	id        int
	email     string
	points    int
	discounts *Discounts
}

func (u *User) ID() int               { return u.id }
func (u *User) Email() string         { return u.email }
func (u *User) Points() int           { return u.points }
func (u *User) Discounts() *Discounts { return u.discounts }

type Discounts struct{ nextOrderDiscount int }

func (c *Discounts) NextOrderDiscount() int { return c.nextOrderDiscount }

func (u *User) UsePoints(points int) error {
	if points <= 0 {
		return errors.New("points must be greater than 0")
	}
	if u.points < points {
		return errors.New("not enough points")
	}
	u.points -= points
	u.discounts.nextOrderDiscount += points
	return nil
}

func unmarshalUser(id int, email string, points int, d *Discounts) *User {
	return &User{id: id, email: email, points: points, discounts: d}
}

func unmarshalDiscounts(n int) *Discounts { return &Discounts{nextOrderDiscount: n} }

// PostgresUserRepository is the outbox-aware variant of the article's
// repository. The only structural change vs. the plain version is
// the closure signature and the loop that inserts events.
type PostgresUserRepository struct {
	db *sql.DB
}

func NewPostgresUserRepository(db *sql.DB) *PostgresUserRepository {
	return &PostgresUserRepository{db: db}
}

// UpdateByID is the outbox version. Compare to the plain UpdateByID
// in the persistence skill (examples/update_fn.go) — the only
// differences are:
//
//   - The closure returns (updated bool, events []any, err error).
//   - After the UPDATE statements, we INSERT each event into the
//     outbox in the same transaction.
//
// Why inserting events here instead of publishing directly: a
// broker publish from inside a database transaction is not atomic
// with the transaction. If the broker call succeeds and then the
// commit fails, you have published an event for a state change that
// never happened. If the commit succeeds and the broker call fails,
// you have lost an event. Writing to the outbox is atomic with the
// commit; the forwarder takes over delivery afterward.
func (r *PostgresUserRepository) UpdateByID(
	ctx context.Context,
	userID int,
	updateFn func(user *User) (bool, []any, error),
) error {
	return runInTx(r.db, func(tx *sql.Tx) error {
		row := tx.QueryRowContext(ctx, "SELECT email, points FROM users WHERE id = $1 FOR UPDATE", userID)

		var email string
		var currentPoints int
		if err := row.Scan(&email, &currentPoints); err != nil {
			return err
		}

		row = tx.QueryRowContext(ctx, "SELECT next_order_discount FROM user_discounts WHERE user_id = $1 FOR UPDATE", userID)

		var discount int
		if err := row.Scan(&discount); err != nil {
			return err
		}

		user := unmarshalUser(userID, email, currentPoints, unmarshalDiscounts(discount))

		updated, events, err := updateFn(user)
		if err != nil {
			return err
		}
		if !updated {
			return nil
		}

		if _, err := tx.ExecContext(ctx,
			"UPDATE users SET email = $1, points = $2 WHERE id = $3",
			user.Email(), user.Points(), user.ID()); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx,
			"UPDATE user_discounts SET next_order_discount = $1 WHERE user_id = $2",
			user.Discounts().NextOrderDiscount(), user.ID()); err != nil {
			return err
		}

		// Insert events into the outbox. The article wraps this as
		// `eventBus, err := NewWatermillEventBus(tx); eventBus.Publish(...)`
		// — same idea, different surface. The point is that the
		// "publish" lands as INSERTs against tx, not as a network
		// call to the broker.
		for _, event := range events {
			if err := insertOutbox(ctx, tx, event); err != nil {
				return err
			}
		}
		return nil
	})
}

// insertOutbox writes one event row. Topic is derived from the Go
// type name (`PointsUsedForDiscount`) so the producer never has to
// hand-spell topic strings; if you prefer explicit topics, accept
// them on the function and have the aggregate return them alongside
// each event.
func insertOutbox(ctx context.Context, tx *sql.Tx, event any) error {
	payload, err := json.Marshal(event)
	if err != nil {
		return err
	}
	topic := topicOf(event)
	_, err = tx.ExecContext(ctx, `
		INSERT INTO outbox_messages (id, topic, payload, metadata)
		VALUES ($1, $2, $3, '{}'::jsonb)
	`, uuid.New(), topic, payload)
	return err
}

func topicOf(event any) string {
	t := reflect.TypeOf(event)
	if t == nil {
		return "events"
	}
	if t.Kind() == reflect.Pointer {
		t = t.Elem()
	}
	if name := t.Name(); name != "" {
		return name
	}
	return "events"
}

// runInTx is the same minimal helper from the article.
func runInTx(db *sql.DB, fn func(tx *sql.Tx) error) error {
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	err = fn(tx)
	if err == nil {
		return tx.Commit()
	}
	if rbErr := tx.Rollback(); rbErr != nil {
		return errors.Join(err, rbErr)
	}
	return err
}
