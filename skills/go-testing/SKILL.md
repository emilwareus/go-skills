---
name: go-testing
description: Write effective Go tests for domain logic, application services, HTTP/gRPC handlers, repositories, concurrency, component flows, integrations, and event-driven systems. Use for Go testing tasks involving table-driven tests, fakes, docker-compose-backed integration tests, real database tests, golden files, race tests, fixtures, t.Parallel, coverage gaps, regression tests, or refactoring code to become testable.
---

# Go Testing

Use this when a Go change needs tests or testability refactoring. Follow local test commands and lint rules first, then choose the smallest test scope that proves the behavior. Keep the everyday path fast enough to run locally, with roughly 10 seconds as a useful target when the repo can support it.

## Test Principles

- **Fast**: keep everyday feedback tight enough to run locally.
- **Enough scenarios on all levels**: cover domain, application, adapter, and component risks at the right scope.
- **Robust and deterministic**: use explicit setup, bounded waits, unique IDs, and stable clocks.
- **Executable locally**: keep the common suite runnable on a laptop.

## Test Scope

Choose the lowest level that can fail for the behavior under review:

1. Domain tests with no IO.
2. Application service tests with fakes for ports.
3. Adapter/integration tests against real dependencies for SQL, serialization, provider clients, broker semantics, or concurrency.
4. Component tests for full in-process service behavior with external edges mocked.
5. End-to-end tests for critical cross-service user workflows.

Do not mock pure domain behavior. Do not unit test implementation details that a refactor should be free to change.

## Workflow

1. Identify the behavior or regression being protected.
2. Choose the smallest test scope that can fail for the right reason.
3. Make time, IDs, randomness, and external dependencies injectable.
4. Use table tests for input/output variation.
5. Use named cases that explain the scenario.
6. Assert observable outcomes.
7. Sabotage the implementation once when a new test is suspicious: temporarily break the guarded behavior and confirm the test fails for the right reason.
8. Run focused tests first, then the package or repo test command.
9. Reduce coverage lint baselines when the repo has them.

## Table Tests

Use table tests when cases share setup and assertions:

```go
func TestInvoiceApprove(t *testing.T) {
    tests := []struct {
        name    string
        invoice Invoice
        wantErr error
    }{
        {name: "draft with lines is approved", invoice: draftInvoice(line("consulting"))},
        {name: "empty draft is rejected", invoice: draftInvoice(), wantErr: ErrInvoiceHasNoLines},
    }

    for i := range tests {
        tc := tests[i]
        t.Run(tc.name, func(t *testing.T) {
            err := tc.invoice.Approve(fixedTime())
            assert.ErrorIs(t, err, tc.wantErr)
        })
    }
}
```

Use slices when order matters. Use maps when order does not matter. For parallel subtests, capture the case with `tc := tests[i]`.

## Fakes

Prefer small fakes owned by the test package for narrow interfaces:

```go
type fakeOrders struct {
    saved []*Order
    err   error
}

func (f *fakeOrders) Save(ctx context.Context, order *Order) error {
    if f.err != nil {
        return f.err
    }
    f.saved = append(f.saved, order)
    return nil
}
```

Use generated mocks when the dependency has many methods, interaction order is the contract, or the repo already has a mock convention.

## Parallel Tests

Use `t.Parallel()` for IO-heavy package, integration, component, API, and slower table subtests when fixtures are parallel-safe. Keep fast domain/unit tests simple unless parallelism measurably helps. If the repo uses the `paralleltest` linter, follow local exemptions and comments; the goal is deliberate parallelism, not mechanical annotations.

Parallel-safe fixtures require:

- Unique IDs, emails, org names, tenant IDs, and idempotency keys.
- Isolated assertions for list queries.
- Fixed timestamps for ordered queries.
- Synchronized access to shared mutable state.
- Targeted cleanup for resources created by the test.

Use `vgt` with `go test -json` when slow test suites appear serialized. Tune package and test parallelism separately:

```sh
go test ./... -parallel 16 -p 4
```

`-parallel` controls how many `t.Parallel()` tests may run at once inside one package. `-p` controls how many packages `go test` runs concurrently. Raising `GOMAXPROCS` above the actual core count can slow the suite through scheduler overhead, so measure before changing it.

## HTTP Handler Tests

Test handlers with `httptest`:

- Build requests with realistic JSON and headers.
- Assert status, response body, and relevant side effects.
- Use fake application services for handler-scope tests.
- Cover malformed JSON, validation failures, auth failures, not found, conflict, and unexpected errors.
- Keep error mapping tests near the transport layer.

## Component Tests

Use component tests when unit tests cannot cover service wiring or in-process behavior:

- Call real HTTP, gRPC, subscriber, or direct-port entry points.
- Use real app/domain/internal adapters.
- Mock external systems owned by other services or providers.
- Assert public behavior: response, persisted state, emitted event, or query result.
- Keep them faster and more focused than E2E tests.

## Repository And Integration Tests

Use real dependencies for persistence behavior:

- SQL constraints, transactions, isolation, locking, migrations, and query mapping.
- Docker-compose-style local database harness or the project's existing equivalent.
- Unique IDs, schemas, or row namespaces as the primary isolation tool.
- Targeted cleanup for resources created by the test.
- Explicit fixtures close to the test.

Do not replace repository tests with mocks that only assert SQL methods were called unless the project explicitly uses that narrow approach.

## Event-Driven Tests

For Pub/Sub, Watermill, or outbox flows:

- Prefer component tests for "event in -> observable state out" and "command in -> event out".
- Use a real local broker or SQL Pub/Sub when Ack/Nack, retry, ordering, or forwarding behavior matters.
- Filter consumed events by unique ID or correlation metadata.
- Use bounded eventual assertions.
- Test duplicate delivery.

## Concurrency Tests

For concurrent code:

- Run `go test -race`.
- Use contexts with deadlines.
- Use channels, wait groups, fake clocks, or eventually assertions for synchronization.
- Test cancellation and shutdown paths.
- Check for goroutine leaks when the project has helpers.

## Anti-Patterns

- Tests that only prove a mock received the same arguments the implementation just assembled.
- `t.Parallel()` added to fast unit tests by habit, increasing scheduling overhead and fixture constraints.
- Shared global fixtures, list-length assertions, or cleanup that deletes data created by other parallel tests.
- Fixed sleeps for async work instead of bounded eventual assertions or synchronization.
- Global truncation cleanup in suites intended to run in parallel.
- Hiding missing tests by adding or preserving coverage-baseline exceptions.
- Raising `GOMAXPROCS` or parallelism flags without measuring the suite.

## Examples

Worked test files live at the repository root in [`tests/`](../../tests/) and are exercised by `go test ./tests/...`:

- [`tests/aggregate/aggregate_test.go`](../../tests/aggregate/aggregate_test.go) - table tests for the `Hour` aggregate, fixture helpers with `t.Helper()`, and `errors.Is`.
- [`tests/handler/handler_test.go`](../../tests/handler/handler_test.go) - application-service tests with hand-written fakes and observable state assertions.
- [`tests/component/component_test.go`](../../tests/component/component_test.go) - event-driven component test using bounded eventual assertions and per-test correlation-ID filtering.
- [`tests/integration/integration_test.go`](../../tests/integration/integration_test.go) - real-database integration test shape for `SELECT ... FOR UPDATE` concurrency behavior.

## Done Criteria

- Tests fail for business regressions.
- Test setup makes dependencies and time explicit.
- Integration tests cover behavior that unit tests would fake incorrectly.
- The selected `go test` command was run, or the blocker is recorded.
- Slow tests have an explicit scope reason.
