// Package integration shows the repository/integration test
// pattern: exercise real persistence behavior against a real
// database. Tests UpdateByID with FOR UPDATE concurrency.
//
// What this test exists to catch — and what unit tests cannot:
//
//   - Real lock behavior. Two goroutines calling UpdateByID for
//     the same user MUST serialize via FOR UPDATE. A unit test
//     with an in-memory map can fake this, but only the real
//     database tells you if the SQL is actually doing it.
//
//   - Constraint and isolation semantics. Unique constraints,
//     CHECK constraints, deferred constraints, default isolation
//     levels — all are database-specific and not reproducible
//     in a mock.
//
//   - Migration compatibility. Running the test against the
//     real, migrated schema catches a stale ALTER TABLE that
//     nobody noticed.
//
// The harness is a placeholder. The cited Three Dots Labs
// integration-testing article uses docker-compose for the local
// database; a real project would use that or its existing harness,
// run migrations once, and give each test its own row space via
// unique IDs.
//
// Lives in tests/ at repo root — see top-level README.
package integration

import (
	"context"
	"database/sql"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestUpdateByID_SerializesConcurrentWrites is the kind of test
// that justifies an integration test: it asserts a property of
// the real SQL behavior under concurrency, which no fake can
// reproduce.
func TestUpdateByID_SerializesConcurrentWrites(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test: requires real database")
	}
	t.Parallel()

	db := openTestDB(t) // docker-compose-backed test DB or shared dev DB
	userID := uniqueUserID(t)
	require.NoError(t, seedUser(t, db, userID, 100))

	repo := NewPostgresUserRepository(db)

	// Two goroutines each try to debit 60 points from a user
	// with 100. Without FOR UPDATE, both could read 100, both
	// would write 40, and the user would lose 60 to a race. With
	// FOR UPDATE the second one queues, sees 40, and (depending
	// on rules) either succeeds with 0 or rejects as
	// insufficient.
	var wg sync.WaitGroup
	errs := make(chan error, 2)
	wg.Add(2)

	for i := 0; i < 2; i++ {
		go func() {
			defer wg.Done()
			errs <- repo.UpdateByID(context.Background(), userID, func(u *User) (bool, error) {
				if u.Points() < 60 {
					return false, errInsufficient
				}
				u.SetPoints(u.Points() - 60)
				return true, nil
			})
		}()
	}
	wg.Wait()
	close(errs)

	// Exactly one of the two should have succeeded, the other
	// should have got insufficient funds. The combined balance
	// at the end must be 40, not 80 (which is what a race
	// would produce) and not -20 (which would mean both wrote).
	var ok, insuf int
	for e := range errs {
		switch {
		case e == nil:
			ok++
		case errors.Is(e, errInsufficient):
			insuf++
		default:
			t.Fatalf("unexpected error: %v", e)
		}
	}
	assert.Equal(t, 1, ok, "exactly one debit should succeed")
	assert.Equal(t, 1, insuf, "exactly one debit should be rejected")

	finalPoints := readUserPoints(t, db, userID)
	assert.Equal(t, 40, finalPoints, "final balance must reflect serialized debits")
}

// ── Fixture isolation ─────────────────────────────────────────────
//
// Unique IDs per test mean two tests running in parallel against
// the same database cannot collide on rows. Combined with
// t.Cleanup for targeted teardown, this is enough to run the
// whole integration suite in parallel safely.
func uniqueUserID(t *testing.T) int {
	t.Helper()
	// In real code: a global atomic counter + a base offset per
	// test process, or a UUID. Keep it tiny and free of locks.
	return int(time.Now().UnixNano() & 0x7FFFFFFF)
}

// ── Stubs ─────────────────────────────────────────────────────────
//
// In a real project these come from the repository package under
// test (PostgresUserRepository from
// skills/go-persistence-transactions/examples/update_fn.go).
// Reproduced minimally here so the file compiles in isolation.

var errInsufficient = errors.New("insufficient points")

type User struct{ id, points int }

func (u *User) Points() int     { return u.points }
func (u *User) SetPoints(p int) { u.points = p }
func (u *User) ID() int         { return u.id }

type PostgresUserRepository struct{ db *sql.DB }

func NewPostgresUserRepository(db *sql.DB) *PostgresUserRepository {
	return &PostgresUserRepository{db: db}
}

// UpdateByID — see the persistence skill for the annotated
// version. Body omitted in this stub.
func (r *PostgresUserRepository) UpdateByID(
	ctx context.Context,
	userID int,
	updateFn func(*User) (bool, error),
) error {
	return errors.New("stub: see skills/go-persistence-transactions/examples/update_fn.go")
}

func openTestDB(t *testing.T) *sql.DB {
	t.Skip("placeholder: real test would open a docker-compose-backed test DB")
	return nil
}

func seedUser(t *testing.T, db *sql.DB, id, points int) error { return nil }
func readUserPoints(t *testing.T, db *sql.DB, id int) int     { return 40 }
