---
name: go-persistence-transactions
description: Implement Go persistence with aggregate repositories, repository-owned transactions, UpdateFn methods, row locking, migrations, tenant scoping, idempotency, and reliable outbox publishing. Use for Go tasks involving database adapters, sqlc, database/sql, pgx, Ent, GORM, Firestore, repository design, transactional application services, consistency boundaries, or reliable event publishing.
---

# Go Persistence And Transactions

Use this when a Go change touches repositories, transactions, migrations, database adapters, idempotency, locking, or outbox publishing. Keep transaction handles inside adapters and make repository methods express business consistency boundaries.

## Repository Shape

Use repositories for aggregate persistence and business queries. The method name should describe the operation the application needs, not the table being touched:

```go
type UserRepository interface {
    UpdateByID(ctx context.Context, userID int, updateFn func(*User) (bool, error)) error
}
```

Repository methods should make consistency needs explicit:

- Load the complete aggregate needed by the business operation.
- Lock rows with `SELECT ... FOR UPDATE` for short critical sections.
- Rehydrate domain objects through trusted constructors.
- Run the callback against the aggregate.
- Persist aggregate changes and commit in the adapter.
- Return domain errors or wrapped infrastructure errors without exposing driver details in the interface.

## Transaction Boundaries

Use one database transaction for one immediate consistency boundary. The preferred shape is a repository-owned UpdateFn method:

```go
err := users.UpdateByID(ctx, cmd.UserID, func(user *User) (bool, error) {
    if err := user.UsePointsAsDiscount(cmd.Points); err != nil {
        return false, err
    }
    return true, nil
})
```

When multiple repositories must commit together, use a transaction provider that passes transaction-bound adapters into the closure:

```go
type TransactionProvider interface {
    Transact(func(Adapters) error) error
}
```

Handlers should receive repositories or adapters, not `*sql.Tx`. Treat `TransactionProvider` as the fallback for workflows that truly need several repositories in one commit. The repository-owned UpdateFn is simpler and safer when one aggregate can own the load/mutate/save sequence.

## UpdateFn Pattern

Use UpdateFn when a command must load, mutate, and save an aggregate consistently:

```go
type AccountRepository interface {
    UpdateByID(ctx context.Context, id AccountID, fn func(*Account) error) error
}
```

The adapter owns transaction start, locking, rehydration, persistence mapping, commit, and rollback. The callback owns domain decisions.

## Anti-Patterns

- `*sql.Tx`, `*gorm.DB`, Firestore transaction handles, or other transaction objects in handler, domain, or repository-interface method signatures.
- Context-based transaction propagation that makes every repository guess whether it is inside a transaction.
- One repository per table when the business operation needs one aggregate boundary.
- Handler-orchestrated `GetX`, `TakeX`, `AddY`, and `SaveX` sequences that should be one repository method.
- Cross-repository transaction blocks that forget row locks such as `SELECT ... FOR UPDATE` on data being changed.
- Holding row locks while calling external APIs, brokers, payment providers, or other services.
- Publishing directly to a broker after a database commit when the event must not be lost.
- Replacing repository integration tests with mocks that only prove a method was called.

## Idempotency

For commands retried by clients, workers, queues, or webhooks:

- Accept or derive an idempotency key.
- Store request identity and final outcome when retries need the same response.
- Use unique constraints for natural de-duplication.
- Treat duplicate delivery as expected behavior.
- Return the same logical result for the same command unless the API documents a different retry contract.

Common cases: payments, webhooks, queue consumers, email sends, and public POST endpoints.

## Locking And Concurrency

Choose one concurrency control per consistency need:

- Unique constraints for "only one can exist" rules.
- Optimistic locking with version columns for user-edited aggregates.
- `SELECT ... FOR UPDATE` for short critical sections.
- Serializable or repeatable-read isolation when the use case needs it and tests cover it.

Keep lock duration short. Keep external network calls outside row locks. Add stress or concurrency tests for lost-update, duplicate-insert, and deadlock-prone paths.

## Outbox Pattern

Use an outbox when domain/application events must be published reliably with a database change:

1. Save aggregate changes and outgoing event rows/messages in one DB transaction.
2. Commit.
3. A forwarder reads pending rows/messages and publishes to the broker.
4. Mark rows published or retry with backoff.
5. Make consumers idempotent.

Store outgoing messages in the same database transaction as the state change, then forward committed messages.

## Mapping Domain And Database

Keep mapping explicit:

- Convert database nulls into domain optional values consciously.
- Validate reconstructed domain objects or use trusted rehydration constructors.
- Keep SQL column names and domain field names free to differ.
- Include tenant, organization, or owner constraints in repository queries.
- Expose database IDs or nullable fields in domain APIs only when they are business concepts.

## Secure Repository Methods

Put visibility-sensitive checks where they cannot be skipped accidentally:

- Model who can see or mutate an aggregate as domain logic.
- Pass the authenticated/domain user into repository methods that load protected data.
- Run the visibility check immediately after rehydration and before returning the aggregate or running `updateFn`.
- Keep tenant, owner, organization, and soft-delete filters inside repository queries when they are part of data access.
- Test "same ID, wrong user/tenant" paths against the repository, not only at the HTTP handler.

For update methods, the repository should load the aggregate, check access, then call the callback:

```go
type TrainingRepository interface {
    UpdateTraining(ctx context.Context, id string, user TrainingUser, updateFn func(*Training) error) error
}
```

This does not move all authorization into persistence. The business rule still belongs in domain code. The repository makes the rule hard to bypass for reads and updates that expose protected data.

## Migrations

For schema changes:

- Follow the repo workflow: model-first, migration-first, or generated migrations.
- Add backward-compatible migrations before code that depends on them when deployments can overlap.
- Backfill large data sets separately from schema changes.
- Add constraints after data is clean.
- Keep already-applied production migrations immutable.

## Adapter Tests

Use real database tests for:

- Query shape and row mapping.
- Tenant isolation.
- Owner/visibility checks on both read and update repository methods.
- Unique constraints and duplicate handling.
- Transactions, locks, and optimistic versions.
- Soft deletes and authorization-sensitive filters.
- Migration compatibility.

Repository tests should prove persistence behavior against the real database engine or the project's accepted local harness.

## Examples

- [`examples/transactor.go`](examples/transactor.go) - `runInTx`, transaction-bound adapters, and `TransactionProvider`.
- [`examples/update_fn.go`](examples/update_fn.go) - `UpdateByID(ctx, userID, func(*User) (bool, error))` with row locks and two-table aggregate persistence.
- [`examples/repository_security.go`](examples/repository_security.go) - repository read/update methods that require a domain user and call `CanUserSeeTraining` after rehydration.

## Done Criteria

- Transaction scope matches one business consistency boundary.
- Repositories express aggregate operations and business queries.
- Retried commands and duplicate messages behave predictably.
- Database-specific behavior is covered by integration tests.
- Migrations and generated schema artifacts follow the repo workflow.
