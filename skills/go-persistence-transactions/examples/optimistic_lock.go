// optimistic_lock.go is a supplementary example: the Three Dots Labs
// article on database transactions reaches for SELECT ... FOR UPDATE
// (pessimistic locking, see update_fn.go). It mentions isolation
// levels as an alternative but does not show the version-column
// approach in code. This file shows that variant against the same
// User aggregate from the article, so you can compare.
//
// When to prefer optimistic locking over FOR UPDATE:
//
//   - Long "think time" between load and save — typically a user
//     editing data in a UI for minutes. Holding a row lock that long
//     blocks every other reader who needs the same row.
//   - Low contention. If two writers rarely collide, retrying on the
//     rare conflict is cheaper than locking on every write.
//
// When FOR UPDATE (as in update_fn.go) is the right choice:
//
//   - Backend handler with no user round-trip: load → mutate → save
//     completes in milliseconds, so the lock is held briefly.
//   - High contention where you'd rather queue than retry —
//     UsePointsAsDiscount on a hot user, for example.
package examples

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
)

// ErrConflict signals that the aggregate moved underneath us. The
// caller either retries (re-load, re-apply, save) or surfaces the
// conflict — e.g. as a 409 to a UI that asks the user to merge.
var ErrConflict = errors.New("optimistic lock conflict: user changed concurrently")

// UserWithVersion mirrors the article's User but adds a version
// counter that the repository owns. The aggregate methods do not
// touch version — only the persistence layer bumps it.
type UserWithVersion struct {
	*User
	version int
}

func (u *UserWithVersion) Version() int { return u.version }

// VersionedUserRepository swaps UpdateByID's pessimistic SELECT
// for an optimistic SELECT (no FOR UPDATE) plus a versioned UPDATE.
type VersionedUserRepository struct {
	db *sql.DB
}

func NewVersionedUserRepository(db *sql.DB) *VersionedUserRepository {
	return &VersionedUserRepository{db: db}
}

// Get loads the aggregate without locking. The version field is the
// caller's proof of "the data I edited was at this version".
func (r *VersionedUserRepository) Get(ctx context.Context, userID int) (*UserWithVersion, error) {
	var (
		email                                 string
		points, discount, version             int
	)
	err := r.db.QueryRowContext(ctx, `
		SELECT u.email, u.points, d.next_order_discount, u.version
		FROM users u
		JOIN user_discounts d ON d.user_id = u.id
		WHERE u.id = $1
	`, userID).Scan(&email, &points, &discount, &version)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("user %d not found", userID)
	}
	if err != nil {
		return nil, fmt.Errorf("load user: %w", err)
	}
	return &UserWithVersion{
		User:    UnmarshalUser(userID, email, points, UnmarshalDiscounts(discount)),
		version: version,
	}, nil
}

// Save commits the caller's edits only if the row's version still
// matches the one the caller read. The race-free check is the
// WHERE version = $4 clause: a concurrent writer would have bumped
// version already, so our UPDATE matches zero rows and we return
// ErrConflict.
//
// We bump version inside the SQL statement (SET version = version + 1)
// rather than computing the new value in Go and writing it back. That
// closes a TOCTOU window: there is no moment where a different
// writer could squeeze in between our read and our write.
func (r *VersionedUserRepository) Save(ctx context.Context, u *UserWithVersion) error {
	return runInTx(r.db, func(tx *sql.Tx) error {
		res, err := tx.ExecContext(ctx, `
			UPDATE users
			SET email = $1, points = $2, version = version + 1
			WHERE id = $3 AND version = $4
		`, u.Email(), u.Points(), u.ID(), u.version)
		if err != nil {
			return fmt.Errorf("save user: %w", err)
		}
		n, err := res.RowsAffected()
		if err != nil {
			return err
		}
		if n == 0 {
			return ErrConflict
		}

		// Discounts goes in the same tx. We don't version it
		// separately because it's part of the same aggregate; the
		// users.version field guards both tables.
		_, err = tx.ExecContext(ctx,
			"UPDATE user_discounts SET next_order_discount = $1 WHERE user_id = $2",
			u.Discounts().NextOrderDiscount(), u.ID())
		if err != nil {
			return fmt.Errorf("save discounts: %w", err)
		}

		u.version++ // reflect the bump so the caller can keep using u
		return nil
	})
}

// Bounded-retry caller pattern. In an interactive flow you usually
// don't retry — you bubble ErrConflict up and let the UI present
// the new version. In a background job (e.g. a worker that processes
// UsePointsAsDiscount commands from a queue) a small retry loop is
// the right shape:
//
//	for attempt := 0; attempt < 3; attempt++ {
//	    u, err := repo.Get(ctx, cmd.UserID)
//	    if err != nil { return err }
//	    if err := u.UsePointsAsDiscount(cmd.Points); err != nil { return err }
//	    err = repo.Save(ctx, u)
//	    if err == nil { return nil }
//	    if !errors.Is(err, ErrConflict) { return err }
//	}
//	return ErrConflict
