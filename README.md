# Go Skills

[![skills.sh](https://skills.sh/b/emilwareus/go-skills)](https://skills.sh/emilwareus/go-skills)

Reusable Go engineering skills for agent-assisted work in Go services. They cover domain modeling, service boundaries, persistence, tests, event-driven workflows, and failure handling.

The skills are not project templates. They tell an agent what to inspect, where code belongs, what tradeoffs to name, and what to verify. They are informed by Go DDD and service architecture practices, including patterns discussed in the Three Dots Labs ecosystem. This project is not affiliated with Three Dots Labs.

## Install

Install all skills from this repository:

```bash
npx skills add emilwareus/go-skills
```

Install one skill:

```bash
npx skills add emilwareus/go-skills --skill go-domain-modeling
```

Install globally for supported agents:

```bash
npx skills add emilwareus/go-skills --global
```

## Skills

| Skill | Use when |
| --- | --- |
| `go-domain-modeling` | Move business state-transition rules into aggregates. |
| `go-service-architecture` | Place code in ports, app, domain, adapters, and composition packages. |
| `go-persistence-transactions` | Design repository-owned transactions, locking, and UpdateFn flows. |
| `go-testing` | Choose unit, application, adapter, component, integration, and event-driven test scopes. |
| `go-errors-observability` | Classify wrapped errors and map stable slug errors at boundaries. |
| `go-event-driven-watermill` | Build Watermill-style event flows and outbox/forwarder patterns. |

## Core Concepts and Source Articles

Each skill teaches a set of patterns drawn from the Three Dots Labs Go ecosystem. Some linked articles are direct sources for mirrored examples; others are supporting references that inform the SKILL.md guidance.

### `go-domain-modeling`

**Core concepts**

- The `Hour` aggregate owns state transitions via business-named methods (`ScheduleTraining`, `CancelTraining`, `MakeAvailable`).
- Unexported fields and named constructors (`NewAvailableHour`, `NewNotAvailableHour`) prevent invalid state from being built casually.
- `Availability` is a typed enum that lives next to the aggregate that owns it.
- Illegal transitions return sentinel errors compared with `errors.Is`.
- Domain code stays free of HTTP, SQL, ORM, brokers, filesystems, frameworks, and telemetry.

**Source articles**

- [Introduction to DDD Lite: When microservices in Go are not enough](https://threedots.tech/post/ddd-lite-in-go-introduction/) — source of the `Hour` aggregate code
- [Combining DDD, CQRS, and Clean Architecture in Go](https://threedots.tech/post/ddd-cqrs-clean-architecture-combined/) — synthesis with the architecture skill
- [Safer Enums in Go](https://threedots.tech/post/safer-enums-in-go/) — typed-int `Availability` pattern
- [Common Anti-Patterns in Go Web Applications](https://threedots.tech/post/common-anti-patterns-in-go-web-applications/) — anemic models and validation-only-at-HTTP

### `go-service-architecture`

**Core concepts**

- Inward dependency direction: ports/transports → app/use cases → domain; adapters/infrastructure → app/use cases → domain.
- Narrow ports defined where consumed, not where implemented (`trainingRepository`, `userService`, `trainerService` in the handler package, not in adapter packages).
- Clean Architecture uses a cohesive multi-method `TrainingService`; the CQRS post splits workflows into command/query handlers (`CancelTrainingHandler`, `AvailableHoursHandler`).
- CQRS split between aggregate-loading command handlers and DTO-returning query handlers reading through `*ReadModel` interfaces.
- `Application{Commands, Queries}` groups the CQRS handlers for composition; handlers still never see `*sql.DB` or a gRPC client.

**Source articles**

- [How to implement Clean Architecture in Go (Golang)](https://threedots.tech/post/introducing-clean-architecture/) — source of the layer vocabulary and multi-method `TrainingService` contrast
- [How to use basic CQRS in Go](https://threedots.tech/post/basic-cqrs-in-go/) — source of the command/query handler shape
- [Combining DDD, CQRS, and Clean Architecture in Go](https://threedots.tech/post/ddd-cqrs-clean-architecture-combined/)
- [When using Microservices or Modular Monolith in Go can be just a detail?](https://threedots.tech/post/microservices-or-monolith-its-detail/)
- [The Best Go framework: no framework?](https://threedots.tech/post/best-go-framework/)
- [Increasing Cohesion in Go with Generic Decorators](https://threedots.tech/post/increasing-cohesion-in-go-with-generic-decorators/) — bus/handler decoration pattern
- [The Over-Engineering Pendulum](https://threedots.tech/post/the-over-engineering-pendulum/) — when not to split
- [Software Dark Ages](https://threedots.tech/post/software-dark-ages/)

### `go-persistence-transactions`

**Core concepts**

- `runInTx` helper owning BEGIN/COMMIT/ROLLBACK so business code never holds a `*sql.Tx`.
- `UpdateFn` repository pattern (`UpdateByID(ctx, id, func(*User) (bool, error))`) — load → mutate → save under one tx, with `SELECT ... FOR UPDATE` for short critical sections.
- `TransactionProvider` + `Adapters` fallback for the case where multiple repositories must commit together; every cross-repo read that relies on locks must remember `FOR UPDATE`.
- Anti-patterns from the article: `*sql.Tx` in method signatures, one-repo-per-table thinking, and handler-orchestrated `GetX`/`TakeX`/`AddY` splits.

**Source articles**

- [Database Transactions in Go with Layered Architecture](https://threedots.tech/post/database-transactions-in-go/) — source of `runInTx`, `UpdateByID`, `TransactionProvider`
- [The Repository pattern in Go](https://threedots.tech/post/repository-pattern-in-go/) — in-memory / MySQL / Firestore variants of the same interface
- [Repository secure by design](https://threedots.tech/post/repository-secure-by-design/) — tenant isolation in queries

### `go-event-driven-watermill`

**Core concepts**

- Events as past-tense facts from the publishing domain; never name an event after a downstream service's action.
- Application boundary: business rules live in command/query handlers; Watermill callbacks translate messages into commands and stop.
- Outbox pattern with Watermill SQL Pub/Sub + Forwarder in the article; this repo's `examples/outbox/*` files are hand-rolled illustrations of the same mechanism.
- The `UpdateByID` outbox variant: closure returns `(bool, []any, error)` so the aggregate decides which events occurred and the repo persists them in the same tx.
- At-least-once delivery is the default contract; consumers must make side effects idempotent.

**Source articles**

- [Distributed Transactions in Go: Read Before You Try](https://threedots.tech/post/distributed-transactions-in-go/) — source of all three handler versions and the outbox `UpdateByID` shape
- [Introducing Watermill — Go event-driven applications library](https://threedots.tech/post/introducing-watermill/)
- [Using MySQL as a Pub/Sub](https://threedots.tech/post/when-sql-database-makes-great-pub-sub/) — motivation for SQL-backed transactional publishing
- [Watermill 1.5 / 1.4 release notes](https://threedots.tech/post/watermill-1-5/) — current API surface for `message.Publisher`/`Subscriber`/`Router`

### `go-testing`

**Core concepts**

- Test scope ladder: domain unit → application service (fakes) → adapter/integration (real deps) → component (real in-process service) → E2E (only critical flows).
- Table tests for input/output variation with named cases; `for i := range tests { tc := tests[i] }` when subtests are parallelized.
- Fakes over generated mocks for narrow interfaces — assert on observable state, not on call sequence.
- Real-database integration tests for SQL constraints, locking, isolation, and migration compatibility, using the article's docker-compose-style local harness.
- Bounded eventual assertions (`assert.EventuallyWithT`) for event-driven flows; per-test correlation IDs to filter parallel tests.
- Deliberate parallelism: `vgt`, `paralleltest`, `-parallel` vs `-p`, unique IDs, and avoiding `t.Parallel()` on fast unit tests.

**Source articles**

- [4 practical principles of high-quality database integration tests in Go](https://threedots.tech/post/database-integration-testing/) — reflected in `tests/integration/`
- [Optimising and Visualising Go Tests Parallelism](https://threedots.tech/post/go-test-parallelism/) — reflected in selective `t.Parallel()` use and fixture conventions
- [Microservices test architecture. Can you sleep well without end-to-end tests?](https://threedots.tech/post/microservices-test-architecture/) — anchors the "component over E2E" stance

### `go-errors-observability`

**Core concepts**

- Sentinel errors and typed slug errors compared with `errors.Is` / `errors.As` — never string matching.
- Wild Workouts-style slug helpers: `NewIncorrectInputError`, `NewAuthorizationError`, `httperr.RespondWithSlugError`, `httperr.InternalError`, and `httperr.Unauthorised`.
- Wrap with `%w` to add operation context; classify and translate at the transport boundary.
- Log unexpected errors once at the transport/worker boundary, never at every stack frame.

**Source articles**

- [Common Anti-Patterns in Go Web Applications](https://threedots.tech/post/common-anti-patterns-in-go-web-applications/) — closest article anchor for logging discipline and error wrapping

This skill is authored synthesis, not an article-mirrored skill. Its examples use Wild Workouts' slug-error and `httperr` concepts, plus general wrapping/logging guidance.

## Repository Layout

```text
skills/                      # everything under here ships via `npx skills add`
  go-domain-modeling/
    SKILL.md
    examples/                # annotated reference code (compiled, see Development)
  go-service-architecture/
    SKILL.md
    examples/
  go-persistence-transactions/
    SKILL.md
    examples/
  go-testing/
    SKILL.md                 # prose only; worked test files live in /tests
  go-errors-observability/
    SKILL.md
    examples/
  go-event-driven-watermill/
    SKILL.md
    examples/
      outbox/                # separate package for the outbox-specific reference impl
tests/                       # not shipped; repo-only worked tests demonstrating go-testing patterns
  aggregate/
  handler/
  component/
  integration/
```

Each skill follows the open Agent Skills format: a directory containing a `SKILL.md` file with YAML frontmatter containing `name` and `description`. The `examples/` directories ship alongside each skill; article-mirrored examples are called out explicitly in the relevant skill docs. Test files live in the top-level `tests/` directory so they are NOT shipped to consumers — they exist to demonstrate the `go-testing` skill's patterns and to keep the test patterns continuously validated by `go test`.

## Development

Validate the skill packaging:

```bash
find skills -maxdepth 3 -name SKILL.md -print
npx skills add . --list
```

Validate the example code compiles and the worked tests pass:

```bash
go vet ./...
go build ./...
go test ./tests/... -short    # -short skips the placeholder integration test
```

The repo is a single Go module (`github.com/emilwareus/go-skills`); each `examples/` directory is a real Go package so signature drift between the article-mirroring code and reality is caught at compile time.

Keep each skill portable. Add examples only when they change a decision an agent would make. Mark project-specific conventions as optional.
