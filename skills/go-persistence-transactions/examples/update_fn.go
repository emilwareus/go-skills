// update_fn.go is the centerpiece pattern from the article: the
// UpdateFn-style repository method.
//
// The contract: the caller supplies a closure that receives a fully
// loaded aggregate and returns (updated bool, err error).
//
//   - The closure makes a domain decision. That is its entire job.
//   - The repository owns BEGIN, the row locks, rehydration from
//     row data into the aggregate, calling the closure, mapping
//     the aggregate back to rows, and COMMIT/ROLLBACK.
//   - The bool tells the repo whether anything actually changed.
//     If the closure returns (false, nil), we skip the UPDATE
//     statements entirely — a cheap optimization and a clean signal
//     for "no-op" cases.
//
// Why this beats handler-owned transactions: the *sql.Tx never
// leaves the repository file. Domain code is a pure function of its
// inputs and stays trivially testable. Forgetting FOR UPDATE on the
// load, or forgetting to commit, becomes impossible at the call site.
package examples

import (
	"context"
	"database/sql"
	"errors"
)

// User is the aggregate from the article. It groups two tables —
// users and user_discounts — under one consistency boundary because
// "use points to add a discount" must be atomic across both.
//
// Notice the field naming: all lowercase, no struct tags, no
// database awareness. Aggregates do not carry their persistence
// metadata; the repository maps between rows and these fields
// explicitly via the Unmarshal* constructors below.
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

// Discounts is the second half of the User aggregate. It lives in
// its own struct (and its own table) but cannot be loaded or saved
// independently of User — that is what makes them one aggregate.
type Discounts struct {
	nextOrderDiscount int
}

func (c *Discounts) NextOrderDiscount() int { return c.nextOrderDiscount }

// UsePointsAsDiscount is the only domain method shown in the
// article. It enforces all of the rules — input validity, balance
// check, balanced double-entry-style update — and either fully
// succeeds or returns an error without touching state.
//
// The repository never reaches inside this method; it just calls it.
// That is the discipline that lets the unit test for this method
// stay trivial: build a User, call the method, assert.
func (u *User) UsePointsAsDiscount(points int) error {
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

// UnmarshalUser is the trusted constructor the repository uses to
// rebuild a User from row data. It bypasses the public surface
// (because the public surface enforces invariants on *changes*, not
// on rehydration of a row that was already valid when it was saved).
// Keeping this in the same package as the aggregate makes the trust
// boundary clear: only the persistence layer in this package may
// call it.
func UnmarshalUser(id int, email string, points int, discounts *Discounts) *User {
	return &User{id: id, email: email, points: points, discounts: discounts}
}

func UnmarshalDiscounts(nextOrderDiscount int) *Discounts {
	return &Discounts{nextOrderDiscount: nextOrderDiscount}
}

// UserRepository is the application-facing interface. UpdateByID is
// the only method the article exposes — that constraint is itself a
// design statement: every mutation goes through load → mutate → save
// under one transaction.
type UserRepository interface {
	UpdateByID(ctx context.Context, userID int, updateFn func(user *User) (bool, error)) error
}

// PostgresUserRepository is the standalone repository from the
// UpdateFn pattern. It owns a *sql.DB, opens its own transaction in
// UpdateByID, and keeps the *sql.Tx inside this file.
type PostgresUserRepository struct {
	db *sql.DB
}

func NewPostgresUserRepository(db *sql.DB) *PostgresUserRepository {
	return &PostgresUserRepository{db: db}
}

// UpdateByID is the body shown in the article. Each step is worth
// reading once:
//
//   - Two SELECT ... FOR UPDATE statements, one per row. Both are
//     locked for the duration of the tx, so two concurrent
//     UsePointsAsDiscount commands for the same user serialize.
//
//   - Both rows are unmarshalled into the User aggregate, then the
//     caller's closure runs against the in-memory User.
//
//   - If the closure says updated == false, we skip the UPDATEs.
//     This handles "the command was a no-op" cleanly — e.g. a
//     handler that decided no change was needed.
//
//   - Two UPDATE statements write the aggregate back. The tx commits
//     in runInTx after this function returns nil.
//
//   - We never call out to anything external while holding these
//     locks. No HTTP, no broker, no slow file write. The whole
//     block is bounded by local DB work.
func (r *PostgresUserRepository) UpdateByID(
	ctx context.Context,
	userID int,
	updateFn func(user *User) (bool, error),
) error {
	return runInTx(r.db, func(tx *sql.Tx) error {
		row := tx.QueryRowContext(ctx, "SELECT email, points FROM users WHERE id = $1 FOR UPDATE", userID)

		var email string
		var currentPoints int
		err := row.Scan(&email, &currentPoints)
		if err != nil {
			return err
		}

		row = tx.QueryRowContext(ctx, "SELECT next_order_discount FROM user_discounts WHERE user_id = $1 FOR UPDATE", userID)

		var discount int
		err = row.Scan(&discount)
		if err != nil {
			return err
		}

		discounts := UnmarshalDiscounts(discount)
		user := UnmarshalUser(userID, email, currentPoints, discounts)

		updated, err := updateFn(user)
		if err != nil {
			return err
		}
		if !updated {
			return nil
		}

		_, err = tx.ExecContext(ctx, "UPDATE users SET email = $1, points = $2 WHERE id = $3", user.Email(), user.Points(), user.ID())
		if err != nil {
			return err
		}

		_, err = tx.ExecContext(ctx, "UPDATE user_discounts SET next_order_discount = $1 WHERE user_id = $2", user.Discounts().NextOrderDiscount(), user.ID())
		if err != nil {
			return err
		}

		return nil
	})
}

// UsePointsAsDiscount is the command. Commands are dumb data
// carriers — no logic, no validation beyond shape — because the
// rules belong on the aggregate.
type UsePointsAsDiscount struct {
	UserID int
	Points int
}

// UsePointsAsDiscountHandler is the article's clean handler. Compare
// the body to the no-transaction version (which had GetPoints +
// TakePoints + AddDiscount as three independent calls, any of which
// could fail mid-way leaving inconsistent state): this version is
// one call, the consistency boundary is in the repository, and the
// caller cannot get it wrong.
type UsePointsAsDiscountHandler struct {
	userRepository UserRepository
}

func (h UsePointsAsDiscountHandler) Handle(ctx context.Context, cmd UsePointsAsDiscount) error {
	return h.userRepository.UpdateByID(ctx, cmd.UserID, func(user *User) (bool, error) {
		err := user.UsePointsAsDiscount(cmd.Points)
		if err != nil {
			return false, err
		}
		return true, nil
	})
}
