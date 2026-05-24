---
name: go-persistence-transactions
description: Implement Go persistence, repositories, migrations, transaction boundaries, aggregate storage, UpdateFn patterns, idempotency, locking, tenant isolation, and outbox/event forwarding. Use for Go tasks involving database adapters, sqlc, database/sql, pgx, Ent, GORM, Firestore, repository design, unit-of-work patterns, transactional application services, consistency boundaries, optimistic concurrency, reliable event publishing, or Three Dots Labs repository/transaction patterns.
---

# Go Persistence And Transactions

Use this when a Go change touches repositories, transactions, migrations, database adapters, idempotency, locking, or outbox publishing. Read local migration and adapter-test rules first.

## Persistence Principles

- Align transaction boundaries with application use cases and aggregate consistency rules.
- Keep domain types independent of database tags and ORM lifecycle hooks unless the project intentionally uses active record.
- Keep repository interfaces narrow and owned by the consuming package or aggregate domain.
- Use migrations according to the repo's documented source of truth.
- Decide idempotency, tenant isolation, and concurrency behavior before writing adapter code.
- Keep database transaction objects out of domain and transport signatures.

## Repository Design

Use repositories for aggregate persistence or specific business queries, not generic table access:

```go
type OrderRepository interface {
    Get(ctx context.Context, id OrderID) (*Order, error)
    Save(ctx context.Context, order *Order) error
}
```

Avoid:

```go
type Repository[T any] interface {
    Create(context.Context, T) error
    Update(context.Context, T) error
    Delete(context.Context, string) error
    List(context.Context) ([]T, error)
}
```

Generic CRUD interfaces hide business queries, locking, tenant isolation, authorization, and consistency requirements.

## Transaction Boundaries

A transaction should cover one application command that needs immediate consistency. Prefer `UpdateByID` when one aggregate can own the load/mutate/save flow. When multiple repositories must commit together, use the article's `TransactionProvider` shape:

```go
type TransactionProvider interface {
    Transact(func(Adapters) error) error
}
```

`TransactionProvider` creates transaction-bound adapters and passes them into the closure. The handler still does not receive a `*sql.Tx`.

Treat this as the risky fallback: every cross-repository `SELECT` that relies on row-lock semantics must remember `FOR UPDATE`. Use the repository-owned `UpdateByID` pattern when it is enough.

Keep external network calls outside database transactions unless the business case explicitly requires holding the lock and the failure mode is understood.

## UpdateFn Pattern

When an aggregate must be loaded, changed, and saved consistently, prefer an UpdateFn-style repository method:

```go
type AccountRepository interface {
    UpdateByID(ctx context.Context, id AccountID, fn func(*Account) error) error
}
```

The adapter owns the transaction, lock, rehydration, persistence mapping, and commit/rollback. The callback owns domain decisions:

```go
err := accounts.UpdateByID(ctx, cmd.AccountID, func(account *Account) error {
    return account.UseCredits(cmd.Amount)
})
```

This keeps `*sql.Tx`, `*gorm.DB`, Firestore transaction handles, and lock details out of application and domain code.

## Idempotency

For commands retried by clients, workers, queues, or webhooks:

- Accept or derive an idempotency key.
- Store request identity and final outcome when retries must receive the same response.
- Use unique constraints for natural de-duplication.
- Treat duplicate delivery as expected behavior.
- Return the same logical result for the same command unless the API documents a different retry contract.

Common cases: payments, webhooks, queue consumers, email sends, and public POST endpoints.

## Locking And Concurrency

Choose deliberately:

- Unique constraints for "only one can exist" rules.
- Optimistic locking with version columns for user-edited aggregates.
- `SELECT ... FOR UPDATE` or equivalent for short critical sections.
- Serializable/repeatable-read isolation only when the use case needs it and tests cover it.

Keep lock duration short. Do not hold locks while calling external APIs. Add stress/concurrency tests for lost-update, duplicate-insert, or deadlock-prone paths.

## Outbox Pattern

Use an outbox when domain/application events must be published reliably with a database change:

1. Save aggregate changes and outgoing event rows/messages in one DB transaction.
2. Commit.
3. A forwarder reads pending rows/messages and publishes to the broker.
4. Mark rows published or retry with backoff.
5. Make consumers idempotent.

Do not publish directly to a broker after commit and assume no event will be lost. Do not publish to a broker inside the transaction and assume rollback/commit will coordinate with the broker. Store the outgoing message in the same database transaction, then forward it.

## Anti-patterns

- Putting `*sql.Tx` in repository interfaces, handlers, or domain method signatures.
- One repository per table when the business operation needs one aggregate boundary.
- Splitting a command into `GetX`, `TakeX`, `AddY` calls orchestrated by the handler.

## Mapping Domain And Database

Keep mapping explicit:

- Convert database nulls into domain optional values consciously.
- Validate reconstructed domain objects or use trusted rehydration constructors.
- Avoid leaking database IDs or nullable fields into domain APIs unless they are business concepts.
- Keep SQL column names and domain field names allowed to differ.
- Keep tenant/org/owner constraints in queries, not only in handlers.

## Migrations

For schema changes:

- Follow the repo workflow: model-first, migration-first, or generated migrations.
- Add backward-compatible migrations before code that depends on them when deployments can overlap.
- Backfill large data sets separately from schema changes when needed.
- Add constraints after data is clean.
- Do not rewrite migrations already applied to production.

## Adapter Tests

Use real database tests for:

- Query shape and row mapping.
- Tenant isolation.
- Unique constraints and duplicate handling.
- Transactions, locks, and optimistic versions.
- Soft deletes and authorization-sensitive filters.
- Migration compatibility.

Avoid mocking a repository to prove a SQL method was called. That does not test persistence behavior.

## Examples

Annotated reference implementations live in `examples/`. They mirror the code from the Three Dots Labs [Database Transactions in Go](https://threedots.tech/post/database-transactions-in-go/) post — same `User` aggregate, `UsePointsAsDiscount` command, and `UpdateByID` signatures — with extra teaching comments so each file reads standalone:

- [`examples/transactor.go`](examples/transactor.go) — the article's `runInTx` helper plus the `TransactionProvider` + `Adapters` pattern for the case where multiple repositories (e.g. `UserRepository` + `AuditLogRepository`) must commit together.
- [`examples/update_fn.go`](examples/update_fn.go) — the article's full `UpdateByID(ctx, userID, func(*User) (bool, error))` body against the `User`/`Discounts` aggregate, with the two `SELECT ... FOR UPDATE` loads and the two-table write.

## Done Criteria

- Transaction scope matches one business consistency boundary.
- Repositories express aggregate operations and business queries.
- Retried commands and duplicate messages behave predictably.
- Database-specific behavior is covered by integration tests.
- Migrations and generated schema artifacts follow the repo's documented workflow.
