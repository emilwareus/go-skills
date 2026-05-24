// Package examples mirrors the code from Three Dots Labs'
// "Database Transactions in Go with Layered Architecture" and
// "Distributed Transactions in Go" posts. The domain (User with
// points and Discounts, command UsePointsAsDiscount) is taken
// directly from those articles so this folder reads as a companion
// to them.
//
// transactor.go covers two pieces:
//
//  1. runInTx — the tiny helper that wraps BEGIN/COMMIT/ROLLBACK so
//     every transactional path has the same shape.
//
//  2. TransactionProvider — the "shared transaction across multiple
//     repositories" pattern used when, e.g., an audit log must be
//     written in the same tx as a domain change. This is the
//     fallback pattern: the article recommends UpdateFn first
//     (see update_fn.go) and only reaches for TransactionProvider
//     when more than one repository genuinely has to commit together.
package examples

import (
	"context"
	"database/sql"
	"errors"
)

// runInTx is the exact helper from the article. The signature is
// deliberately minimal: no context, no isolation options. Both can
// be threaded through later if the project needs them, but the small
// shape is part of what makes this safe — there is nothing to
// misuse. The contract:
//
//   - If fn returns nil, COMMIT and return its (nil) error.
//   - If fn returns an error, ROLLBACK; if rollback itself fails,
//     join both errors so the post-mortem sees both. Using
//     errors.Join (Go 1.20+) is what the article uses too.
//
// Notice what is NOT here: no panic recovery. If a handler panics,
// the connection's deferred close will eventually roll the tx back,
// but the article keeps the helper minimal on purpose — surprises
// belong in your code, not in your helper.
func runInTx(db *sql.DB, fn func(tx *sql.Tx) error) error {
	tx, err := db.Begin()
	if err != nil {
		return err
	}

	err = fn(tx)
	if err == nil {
		return tx.Commit()
	}

	rollbackErr := tx.Rollback()
	if rollbackErr != nil {
		return errors.Join(err, rollbackErr)
	}

	return err
}

// db is the minimal interface that *sql.Tx satisfies. Repositories
// created inside TransactionProvider accept this so the handler sees
// repositories, not the concrete transaction object.
type db interface {
	QueryRowContext(ctx context.Context, query string, args ...interface{}) *sql.Row
	ExecContext(ctx context.Context, query string, args ...interface{}) (sql.Result, error)
}

// PostgresTxUserRepository is the tx-bound variant used only by
// TransactionProvider. It is intentionally separate from
// PostgresUserRepository in update_fn.go, because that type owns a
// *sql.DB and opens its own transaction.
type PostgresTxUserRepository struct {
	db db
}

func NewPostgresTxUserRepository(db db) *PostgresTxUserRepository {
	return &PostgresTxUserRepository{db: db}
}

func (r *PostgresTxUserRepository) UpdateByID(
	ctx context.Context,
	userID int,
	updateFn func(user *User) (bool, error),
) error {
	row := r.db.QueryRowContext(ctx, "SELECT email, points FROM users WHERE id = $1 FOR UPDATE", userID)

	var email string
	var currentPoints int
	err := row.Scan(&email, &currentPoints)
	if err != nil {
		return err
	}

	row = r.db.QueryRowContext(ctx, "SELECT next_order_discount FROM user_discounts WHERE user_id = $1 FOR UPDATE", userID)

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

	_, err = r.db.ExecContext(ctx, "UPDATE users SET email = $1, points = $2 WHERE id = $3", user.Email(), user.Points(), user.ID())
	if err != nil {
		return err
	}

	_, err = r.db.ExecContext(ctx, "UPDATE user_discounts SET next_order_discount = $1 WHERE user_id = $2", user.Discounts().NextOrderDiscount(), user.ID())
	return err
}

// PostgresAuditLogRepository is the second repository used to motivate
// TransactionProvider: writing an audit log alongside the domain
// change, both committed together or both rolled back.
type PostgresAuditLogRepository struct {
	db db
}

func NewPostgresAuditLogRepository(db db) *PostgresAuditLogRepository {
	return &PostgresAuditLogRepository{db: db}
}

func (r *PostgresAuditLogRepository) StoreAuditLog(ctx context.Context, log string) error {
	_, err := r.db.ExecContext(ctx, "INSERT INTO audit_log (entry) VALUES ($1)", log)
	return err
}

// Adapters bundles the repositories the application handler needs.
// TransactionProvider produces a fresh Adapters struct per tx, with
// every repository pre-bound to that tx — so application code uses
// adapters.UserRepository as if it were a normal repo, and the fact
// that a transaction is in play is hidden.
type Adapters struct {
	UserRepository     UserRepository
	AuditLogRepository AuditLogRepository
}

// AuditLogRepository is the small interface application code depends
// on — concrete *sql.Tx never appears in the handler.
type AuditLogRepository interface {
	StoreAuditLog(ctx context.Context, log string) error
}

// txProvider is the interface the handler depends on. Note: the
// closure takes Adapters, not a *sql.Tx. The handler stays unaware
// of the database driver.
type txProvider interface {
	Transact(txFunc func(adapters Adapters) error) error
}

// TransactionProvider is the concrete implementation.
type TransactionProvider struct {
	db *sql.DB
}

func NewTransactionProvider(db *sql.DB) *TransactionProvider {
	return &TransactionProvider{db: db}
}

// Transact runs txFunc inside one transaction, with both repositories
// bound to that transaction. The handler's body can call
// adapters.UserRepository.UpdateByID(...) and
// adapters.AuditLogRepository.StoreAuditLog(...) and trust that they
// commit together — or neither does.
//
// The article calls this out as the fallback pattern: it is more
// fragile than the UpdateFn-only approach because every SELECT in
// every repository must remember to use FOR UPDATE for the row-lock
// guarantees to hold across the whole tx. Use it when you genuinely
// need cross-repository atomicity (e.g. audit log + domain write),
// not as a default.
func (p *TransactionProvider) Transact(txFunc func(adapters Adapters) error) error {
	return runInTx(p.db, func(tx *sql.Tx) error {
		adapters := Adapters{
			UserRepository:     NewPostgresTxUserRepository(tx),
			AuditLogRepository: NewPostgresAuditLogRepository(tx),
		}
		return txFunc(adapters)
	})
}

// Handler shape, mirroring the article. The handler depends on
// txProvider — not *sql.DB, not *sql.Tx, not the concrete
// repositories. Swapping the implementation (in-memory for tests,
// Postgres for prod) is a wiring change, not a code change.
//
//	type UsePointsAsDiscountHandler struct {
//	    txProvider txProvider
//	}
//
//	func (h UsePointsAsDiscountHandler) Handle(ctx context.Context, cmd UsePointsAsDiscount) error {
//	    return h.txProvider.Transact(func(adapters Adapters) error {
//	        err := adapters.UserRepository.UpdateByID(ctx, cmd.UserID, func(user *User) (bool, error) {
//	            return true, user.UsePointsAsDiscount(cmd.Points)
//	        })
//	        if err != nil {
//	            return fmt.Errorf("could not use points as discount: %w", err)
//	        }
//
//	        log := fmt.Sprintf("used %d points as discount for user %d", cmd.Points, cmd.UserID)
//	        return adapters.AuditLogRepository.StoreAuditLog(ctx, log)
//	    })
//	}
