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
| `go-strategic-ddd` | Decide bounded contexts, context integration, ubiquitous language, and service/module splits. |
| `go-domain-modeling` | Move business state-transition rules into aggregates. |
| `go-service-architecture` | Place code in ports, app, domain, adapters, and composition packages. |
| `go-code-quality-patterns` | Apply deliberate duplication, safe enums, value objects, decorators, and low-coupling library choices. |
| `go-persistence-transactions` | Design repository-owned transactions, locking, and UpdateFn flows. |
| `go-event-driven-watermill` | Build Watermill-style event flows and outbox/forwarder patterns. |
| `go-testing` | Choose unit, application, adapter, component, integration, and event-driven test scopes. |
| `go-service-platform` | Keep local dev, generated API contracts, auth, Terraform, Cloud Run-style deployment, and CI wiring at the edge. |
| `go-errors-observability` | Classify wrapped errors and map stable slug errors at boundaries. |

## Coverage

The skills are organized by engineering decision, not by source chronology:

| Area | Skills |
| --- | --- |
| Strategic DDD and context boundaries | `go-strategic-ddd`, `go-service-architecture` |
| Tactical DDD and aggregates | `go-domain-modeling`, `go-persistence-transactions` |
| Repository pattern and transactions | `go-persistence-transactions`, `go-testing` |
| CQRS and Clean Architecture | `go-service-architecture`, `go-testing` |
| Event-driven and distributed consistency | `go-event-driven-watermill`, `go-persistence-transactions` |
| Testing architecture and CI harnesses | `go-testing`, `go-service-platform` |
| Delivery platform and generated contracts | `go-service-platform`, `go-service-architecture` |
| Go code quality patterns | `go-code-quality-patterns`, `go-errors-observability` |

## Core Concepts

Each skill teaches reusable Go patterns. The reference links show where many of the patterns came from, but the skills themselves are written as operating guidance rather than article summaries.

### `go-strategic-ddd`

**Core concepts**

- Reconstruct business flows from domain events, commands, actors, policies, read models, aggregates, and pivotal events.
- Bounded contexts own one model and one vocabulary; deployment as monolith or microservices comes later.
- Cross-context communication uses direct ports, public APIs, domain events, shared kernels, or anti-corruption adapters intentionally.
- Context splits should be explained through business capability and consistency needs, not tables or endpoints.

**References**

- [DDD Knowledge Index](https://academy.threedots.tech/knowledge/ddd/)
- [Strategic Domain Design Knowledge Index](https://academy.threedots.tech/knowledge/strategic-domain-design/)
- [Software Dark Ages](https://threedots.tech/post/software-dark-ages/)
- [When using Microservices or Modular Monolith in Go can be just a detail?](https://threedots.tech/post/microservices-or-monolith-its-detail/)
- [When to avoid DRY in Go](https://threedots.tech/post/things-to-know-about-dry/)
- [Combining DDD, CQRS, and Clean Architecture in Go](https://threedots.tech/post/ddd-cqrs-clean-architecture-combined/)

### `go-domain-modeling`

**Core concepts**

- The `Hour` aggregate owns state transitions via business-named methods (`ScheduleTraining`, `CancelTraining`, `MakeAvailable`).
- Unexported fields and named constructors (`NewAvailableHour`, `NewNotAvailableHour`) prevent invalid state from being built casually.
- `Availability` is a typed enum that lives next to the aggregate that owns it.
- Illegal transitions return sentinel errors compared with `errors.Is`.
- Domain code stays free of HTTP, SQL, ORM, brokers, filesystems, frameworks, and telemetry.

**References**

- [Introduction to DDD Lite: When microservices in Go are not enough](https://threedots.tech/post/ddd-lite-in-go-introduction/)
- [Combining DDD, CQRS, and Clean Architecture in Go](https://threedots.tech/post/ddd-cqrs-clean-architecture-combined/)
- [Safer Enums in Go](https://threedots.tech/post/safer-enums-in-go/)
- [Common Anti-Patterns in Go Web Applications](https://threedots.tech/post/common-anti-patterns-in-go-web-applications/)

### `go-service-architecture`

**Core concepts**

- Inward dependency direction: ports/transports -> app/use cases -> domain; adapters/infrastructure -> app/use cases -> domain.
- Narrow ports defined where consumed, not where implemented (`trainingRepository`, `userService`, `trainerService` in the handler package, not in adapter packages).
- Clean Architecture uses a cohesive multi-method `TrainingService`; the CQRS post splits workflows into command/query handlers (`CancelTrainingHandler`, `AvailableHoursHandler`).
- CQRS split between aggregate-loading command handlers and DTO-returning query handlers reading through `*ReadModel` interfaces.
- `Application{Commands, Queries}` groups the CQRS handlers for composition; handlers still never see `*sql.DB` or a gRPC client.

**References**

- [How to implement Clean Architecture in Go (Golang)](https://threedots.tech/post/introducing-clean-architecture/)
- [How to use basic CQRS in Go](https://threedots.tech/post/basic-cqrs-in-go/)
- [Combining DDD, CQRS, and Clean Architecture in Go](https://threedots.tech/post/ddd-cqrs-clean-architecture-combined/)
- [When using Microservices or Modular Monolith in Go can be just a detail?](https://threedots.tech/post/microservices-or-monolith-its-detail/)
- [The Best Go framework: no framework?](https://threedots.tech/post/best-go-framework/)
- [Increasing Cohesion in Go with Generic Decorators](https://threedots.tech/post/increasing-cohesion-in-go-with-generic-decorators/)
- [The Over-Engineering Pendulum](https://threedots.tech/post/the-over-engineering-pendulum/)
- [Software Dark Ages](https://threedots.tech/post/software-dark-ages/)

### `go-code-quality-patterns`

**Core concepts**

- Duplicate data shapes when API responses, database models, event payloads, generated types, and domain objects change for different reasons.
- Use constructors, private fields, and safe enums to keep important business values valid in memory.
- Use decorators for authorization, logging, metrics, tracing, and retry behavior around command/query handlers.
- Prefer small composable libraries over frameworks that force domain/application code to follow framework conventions.
- Add abstractions only when they remove real coupling, improve tests, or clarify business rules.

**References**

- [When to avoid DRY in Go](https://threedots.tech/post/things-to-know-about-dry/)
- [Safer Enums in Go](https://threedots.tech/post/safer-enums-in-go/)
- [Increasing Cohesion in Go with Generic Decorators](https://threedots.tech/post/increasing-cohesion-in-go-with-generic-decorators/)
- [Common Anti-Patterns in Go Web Applications](https://threedots.tech/post/common-anti-patterns-in-go-web-applications/)
- [The Best Go framework: no framework?](https://threedots.tech/post/best-go-framework/)
- [The Go libraries that never failed us](https://threedots.tech/post/list-of-recommended-libraries/)
- [The Over-Engineering Pendulum](https://threedots.tech/post/the-over-engineering-pendulum/)

### `go-persistence-transactions`

**Core concepts**

- `runInTx` helper owning BEGIN/COMMIT/ROLLBACK so business code never holds a `*sql.Tx`.
- `UpdateFn` repository pattern (`UpdateByID(ctx, id, func(*User) (bool, error))`) - load -> mutate -> save under one tx, with `SELECT ... FOR UPDATE` for short critical sections.
- `TransactionProvider` + `Adapters` fallback for the case where multiple repositories must commit together; every cross-repo read that relies on locks must remember `FOR UPDATE`.
- Secure-by-design repositories pass the domain user into protected reads/updates and call domain visibility checks after rehydration.
- Anti-patterns to catch: `*sql.Tx` in method signatures, one-repo-per-table thinking, and handler-orchestrated `GetX`/`TakeX`/`AddY` splits.

**References**

- [Database Transactions in Go with Layered Architecture](https://threedots.tech/post/database-transactions-in-go/)
- [The Repository pattern in Go](https://threedots.tech/post/repository-pattern-in-go/)
- [Repository secure by design](https://threedots.tech/post/repository-secure-by-design/)

### `go-event-driven-watermill`

**Core concepts**

- Events as past-tense facts from the publishing domain; never name an event after a downstream service's action.
- Application boundary: business rules live in command/query handlers; Watermill callbacks translate messages into commands and stop.
- Distributed consistency decision: when the business accepts waiting, publish events and retry internally instead of forcing distributed transactions.
- Watermill CQRS `EventBus` / `EventProcessor` setup stays in adapters/composition; typed handlers map events to commands.
- Outbox pattern with Watermill SQL Pub/Sub + Forwarder; this repo's `examples/outbox/*` files are hand-rolled illustrations of the same mechanism.
- The `UpdateByID` outbox variant: closure returns `(bool, []any, error)` so the aggregate decides which events occurred and the repo persists them in the same tx.
- Durable execution: persist validated input, acknowledge after durable side effects, keep handler state changes atomic, and prove idempotency with duplicate-message tests.
- At-least-once delivery is the default contract; consumers must expose poison/dead-letter handling when messages fail repeatedly.

**References**

- [Distributed Transactions in Go: Read Before You Try](https://threedots.tech/post/distributed-transactions-in-go/)
- [Introducing Watermill - Go event-driven applications library](https://threedots.tech/post/introducing-watermill/)
- [Using MySQL as a Pub/Sub](https://threedots.tech/post/when-sql-database-makes-great-pub-sub/)
- [Durable Background Execution with Go and SQLite](https://threedots.tech/post/sqlite-durable-execution/)
- [Watermill 1.5 / 1.4 release notes](https://threedots.tech/post/watermill-1-5/)

### `go-testing`

**Core concepts**

- Test scope ladder: domain unit -> application service (fakes) -> adapter/integration (real deps) -> component (real in-process service) -> E2E (only critical flows).
- Table tests for input/output variation with named cases; `for i := range tests { tc := tests[i] }` when subtests are parallelized.
- Fakes over generated mocks for narrow interfaces - assert on observable state, not on call sequence.
- Real-database integration tests for SQL constraints, locking, isolation, and migration compatibility, using a docker-compose-style local harness or the repo's equivalent.
- Bounded eventual assertions (`assert.EventuallyWithT`) for event-driven flows; per-test correlation IDs to filter parallel tests.
- Deliberate parallelism: `vgt`, `paralleltest`, `-parallel` vs `-p`, unique IDs, and avoiding `t.Parallel()` on fast unit tests.
- Docker Compose CI pattern: reuse local topology, add CI overrides, test built images, then deploy.

**References**

- [4 practical principles of high-quality database integration tests in Go](https://threedots.tech/post/database-integration-testing/)
- [Optimising and Visualising Go Tests Parallelism](https://threedots.tech/post/go-test-parallelism/)
- [Microservices test architecture. Can you sleep well without end-to-end tests?](https://threedots.tech/post/microservices-test-architecture/)
- [Running integration tests with docker-compose in Google Cloud Build](https://threedots.tech/post/running-integration-tests-on-google-cloud-build/)

### `go-service-platform`

**Core concepts**

- Local development should run the service graph with Docker Compose and keep hot reload as a development-only concern.
- HTTP ports use small routers, standard middleware, and generated OpenAPI server interfaces; handlers call application commands/queries.
- gRPC ports use `.proto` contracts and generated server/client code; generated types are translated at the boundary.
- Auth provider details stay at the edge: middleware verifies tokens and produces a small authenticated user value for application code.
- Terraform/CI changes are reviewed as code; CI should test the images or service graph that will be deployed.

**References**

- [Building a serverless application with Go, Google Cloud Run and Firebase](https://threedots.tech/post/serverless-cloud-run-firebase-modern-go-application/)
- [A complete Terraform setup of a serverless application on Google Cloud Run and Firebase](https://threedots.tech/post/complete-setup-of-serverless-application/)
- [Robust gRPC communication on Google Cloud Run](https://threedots.tech/post/robust-grpc-google-cloud-run/)
- [You should not build your own authentication](https://threedots.tech/post/firebase-cloud-run-authentication/)
- [Running integration tests with docker-compose in Google Cloud Build](https://threedots.tech/post/running-integration-tests-on-google-cloud-build/)
- [Creating local Go dev environment with Docker and live code reloading](https://threedots.tech/post/go-docker-dev-environment-with-go-modules-and-live-code-reloading/)

### `go-errors-observability`

**Core concepts**

- Sentinel errors and typed slug errors compared with `errors.Is` / `errors.As` - never string matching.
- Wild Workouts-style slug helpers: `NewIncorrectInputError`, `NewAuthorizationError`, `httperr.RespondWithSlugError`, `httperr.InternalError`, and `httperr.Unauthorised`.
- Wrap with `%w` to add operation context; classify and translate at the transport boundary.
- Log unexpected errors once at the transport/worker boundary, never at every stack frame.

**References**

- [Common Anti-Patterns in Go Web Applications](https://threedots.tech/post/common-anti-patterns-in-go-web-applications/)

This skill is authored synthesis. Its examples use Wild Workouts' slug-error and `httperr` concepts, plus general wrapping/logging guidance.

## Repository Layout

```text
skills/                      # everything under here ships via `npx skills add`
  go-strategic-ddd/
    SKILL.md
  go-domain-modeling/
    SKILL.md
    examples/                # annotated reference code (compiled, see Development)
  go-service-architecture/
    SKILL.md
    examples/
  go-code-quality-patterns/
    SKILL.md
    examples/
  go-persistence-transactions/
    SKILL.md
    examples/
  go-event-driven-watermill/
    SKILL.md
    examples/
      outbox/                # separate package for the outbox-specific reference impl
  go-testing/
    SKILL.md                 # prose only; worked test files live in /tests
  go-service-platform/
    SKILL.md
  go-errors-observability/
    SKILL.md
    examples/
tests/                       # not shipped; repo-only worked tests demonstrating go-testing patterns
  aggregate/
  handler/
  component/
  integration/
```

Each skill follows the open Agent Skills format: a directory containing a `SKILL.md` file with YAML frontmatter containing `name` and `description`. The `examples/` directories ship alongside each skill and demonstrate the relevant patterns. Test files live in the top-level `tests/` directory so they are NOT shipped to consumers - they exist to demonstrate the `go-testing` skill's patterns and to keep the test patterns continuously validated by `go test`.

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

The repo is a single Go module (`github.com/emilwareus/go-skills`); each `examples/` directory is a real Go package so signature drift in examples is caught at compile time.

Keep each skill portable. Add examples only when they change a decision an agent would make. Mark project-specific conventions as optional.
