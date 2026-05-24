---
name: go-testing
description: Write effective Go tests for domain logic, application services, HTTP/gRPC handlers, repositories, concurrency, component flows, integrations, and event-driven systems. Use for Go testing tasks involving table-driven tests, fakes versus mocks, docker-compose-backed integration tests, real database tests, golden files, race tests, fixtures, t.Parallel, coverage gaps, regression tests, or refactoring code to become testable without over-mocking.
---

# Go Testing

Use this when a Go change needs tests or testability refactoring. Read local test docs first; repo-specific gates such as `make test`, `make core-check`, `paralleltest`, adapter-test lints, or component-test lints override this guidance.

## Four Principles

The database integration testing article leads with four principles:

- **Fast**: keep feedback tight enough that developers actually run the suite.
- **Testing enough scenarios on all levels**: cover domain, application, adapter, and component risks at the right scope.
- **Robust and deterministic**: no accidental sleeps, order dependencies, or shared-state flakes.
- **Executable locally**: the common suite should run on a laptop, with about a 10s target for the everyday path.

## Testing Strategy

Choose the lowest test level that can fail for the behavior under review:

1. Domain tests with no IO.
2. Application service tests with fakes for ports when orchestration branches or error handling matters.
3. Adapter/integration tests against real dependencies when behavior depends on SQL, serialization, provider clients, broker semantics, or concurrency.
4. Component tests for full in-process service behavior with external edges mocked.
5. End-to-end tests only for critical cross-service user workflows.

Do not mock pure domain behavior. Do not unit test implementation details that a refactor should be free to change. Do not add tests that only prove the mock received the same arguments as the code under test.

## Workflow

1. Identify the behavior or regression being protected.
2. Choose the smallest test scope that can fail for the right reason.
3. Make time, IDs, randomness, and external dependencies injectable.
4. Use table tests for input/output variation.
5. Use named cases that explain the scenario.
6. Assert observable outcomes, not private call sequences unless the sequence is the contract.
7. Sabotage the implementation once when the test is new or suspicious: temporarily break the guarded behavior and confirm the test fails for the right reason.
8. Run focused tests first, then the package or repo test command.
9. If the repo has coverage lint baselines, reduce them; do not add new baseline entries to hide missing tests.

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

Use slices when order matters; use maps when it does not. For parallel subtests, the `for i := range tests { tc := tests[i] }` shape stays obvious across Go versions.

## Fakes Over Mocks

Prefer small fakes owned by the test package:

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

Use mocks when:

- The dependency has many methods and a generated mock already exists.
- The interaction order is part of the behavior.
- The project has an established mocking convention.

Avoid tests that only prove "method X called method Y" for ordinary orchestration.

## Parallel Tests

Use `t.Parallel()` for IO-heavy package, integration, component, API, and slower table subtests when fixtures are parallel-safe. Do not add `t.Parallel()` to fast unit/domain tests just to satisfy a habit; the scheduling overhead and fixture constraints can outweigh any win.

If the repo uses the `paralleltest` linter, follow its local exemptions and comments. The article's point is deliberate parallelism, not mechanically marking every test.

Parallel-safe fixtures require:

- Unique IDs, emails, org names, tenant IDs, and idempotency keys.
- No assertions on global list length unless isolated.
- Fixed timestamps for ordered queries.
- No shared mutable package state without synchronization.
- Cleanup that targets only resources created by the test.

Use `vgt` (Visualizing Go Tests) with `go test -json` when slow test suites appear serialized despite many cores. Distinguish the knobs:

```sh
go test ./... -parallel 16 -p 4
```

`-parallel` controls how many `t.Parallel()` tests may run at once inside one package. `-p` controls how many packages `go test` runs concurrently. Raising `GOMAXPROCS` above the actual core count can slow the suite down through scheduler overhead; measure before changing it.

## HTTP Handler Tests

Test handlers with `httptest`:

- Build requests with realistic JSON and headers.
- Assert status, response body, and relevant side effects.
- Use fake application services rather than real databases unless this is an integration/component test.
- Cover malformed JSON, validation failures, auth failures, not found, conflict, and unexpected errors.

Keep error mapping tests near the transport layer.

## Component Tests

Use component tests when unit tests cannot cover service wiring or in-process behavior:

- Call real HTTP/gRPC/subscriber/direct-port entry points.
- Use real app/domain/internal adapters unless they cross process, network, or provider boundaries.
- Mock only external systems owned by other services or providers.
- Assert public behavior: response, persisted state, emitted event, or query result.
- Keep them faster and more focused than E2E tests.

## Repository And Integration Tests

Use real dependencies for persistence behavior:

- SQL constraints, transactions, isolation, locking, migrations, and query mapping need integration tests.
- Prefer the article's docker-compose-style local database harness or the project's existing equivalent.
- Use unique IDs, schemas, or row namespaces as the primary isolation tool.
- Prefer targeted cleanup for resources created by the test; avoid global truncation/cleanup unless the suite is deliberately serialized.
- Keep fixtures explicit and close to the test unless shared fixtures are already well-designed.

Do not replace repository tests with mocks that assert SQL strings unless the project explicitly uses sqlmock for a narrow reason.

## Event-Driven Tests

For Pub/Sub, Watermill, or outbox flows:

- Prefer component tests for "event in -> observable state out" and "command in -> event out".
- Use real local broker/SQL PubSub when Ack/Nack, retry, ordering, or forwarding behavior matters.
- Filter consumed events by unique ID or correlation metadata.
- Use bounded eventual assertions instead of fixed sleeps.
- Test idempotent consumers with duplicate messages.

## Concurrency Tests

For concurrent code:

- Run `go test -race`.
- Use contexts with deadlines.
- Avoid sleeps as synchronization. Prefer channels, wait groups, fake clocks, or eventually assertions with bounded timeouts.
- Test cancellation and shutdown paths.
- Check for goroutine leaks when the project has helpers for it.

## Examples

Worked test files demonstrating the patterns above live at the repository root in [`tests/`](../../tests/) rather than inside this skill, so they are not shipped when an agent installs the skill. They are kept in the repo for reference and to be exercised by `go test ./tests/...`:

- [`tests/aggregate/aggregate_test.go`](../../tests/aggregate/aggregate_test.go) — table tests for the `Hour` aggregate (`ScheduleTraining`), fixture helpers with `t.Helper()`, `errors.Is` over string matching.
- [`tests/handler/handler_test.go`](../../tests/handler/handler_test.go) — application-service tests for `CancelTrainingHandler` using hand-written fakes (not mocks) that record state; assertions are on observable outcomes, not call sequences.
- [`tests/component/component_test.go`](../../tests/component/component_test.go) — event-driven component test using `assert.EventuallyWithT` for bounded eventual assertions and per-test correlation-ID filtering for parallel safety.
- [`tests/integration/integration_test.go`](../../tests/integration/integration_test.go) — real-database integration test that proves `SELECT ... FOR UPDATE` actually serializes concurrent writes, with unique IDs per test for fixture isolation.

## Done Criteria

- Tests fail for business regressions, not incidental refactors.
- Test setup makes dependencies and time explicit.
- Integration tests cover behavior that unit tests would fake incorrectly.
- The selected `go test` command was run, or the blocker is recorded.
- Slow or flaky tests have an explicit scope reason, not accidental sleeps or shared fixtures.
