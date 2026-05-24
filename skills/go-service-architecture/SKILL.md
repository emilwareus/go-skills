---
name: go-service-architecture
description: Structure Go services with clear package boundaries, dependency direction, ports, application use cases, adapters, direct ports, events, and dependency injection. Use for Go backend architecture, Clean Architecture, Hexagonal Architecture, DDD service layout, CQRS command/query surfaces, refactoring tangled packages, introducing ports/adapters, or deciding where HTTP, gRPC, CLI, queues, config, domain, app, adapter, and service wiring code belong.
---

# Go Service Architecture

Use this when package boundaries, dependency direction, or service wiring are part of a Go change. Read local docs such as `ARCHITECTURE.md`, `CODE_PATTERNS.md`, `CLAUDE.md`, `AGENTS.md`, or `TESTING_GUIDELINES.md` before moving code.

## Architecture Rule

Dependencies point inward:

```text
ports/transports -> app/use cases -> domain
adapters/infrastructure -> app/use cases -> domain
service/cmd composition wires everything
```

The Three Dots Labs vocabulary is:

- **Domain**: business rules and domain types.
- **Application**: commands, queries, and use-case orchestration.
- **Ports**: transport-facing entry points such as HTTP, gRPC, CLI, or message handlers.
- **Adapters**: database, external-service, Pub/Sub, and other infrastructure implementations.

The domain must not know about HTTP, gRPC, SQL, queues, config, logging libraries, telemetry vendors, filesystem/process execution, or process lifecycle. The application layer can orchestrate those capabilities only through narrow interfaces.

## Decision Workflow

1. Inspect the current package layout before adding a pattern.
2. Identify the workflow: command, query, background job, integration event, direct port, or migration.
3. Put transport parsing and response mapping at the edge.
4. Put orchestration in an application service, command handler, query handler, or use case.
5. Put business decisions in domain types.
6. Put SQL, external APIs, queues, clocks, random IDs, file systems, and process execution behind narrow interfaces owned by the package that consumes them.
7. Wire concrete dependencies in `main`, `cmd`, `service`, or the repo's established composition package.
8. Add or update component tests for new wiring.

## Practical Package Layout

Small services can stay simple:

```text
cmd/api/main.go
internal/orders/
  order.go
  service.go
  repository.go
  http.go
  postgres.go
```

Split adapters when one package now contains multiple transports, multiple infrastructure adapters, or import rules the repo enforces mechanically:

```text
internal/orders/
  domain/
  app/
  adapters/postgres/
  ports/httpapi/
  service/
```

Avoid copying a large template into a small service. Add folders when the current package now has mixed responsibilities or the repo already uses that boundary.

## Application Services

Application services coordinate IO, authorization, transaction scope, idempotency, logging boundaries, and domain calls. Keep each handler focused on one workflow:

```go
type CancelTrainingHandler struct {
    repo           trainingRepository
    userService    userService
    trainerService trainerService
}

func (h CancelTrainingHandler) Handle(ctx context.Context, cmd CancelTraining) error {
    return h.repo.CancelTraining(ctx, cmd.TrainingUUID, func(ctx context.Context, tr *Training) error {
        if err := tr.Cancel(); err != nil {
            return err
        }
        if err := h.trainerService.CancelTraining(ctx, tr.Time); err != nil {
            return fmt.Errorf("cancel trainer schedule: %w", err)
        }
        return nil
    })
}
```

Do not default to generic "manager" objects. The Clean Architecture article does use a cohesive multi-method `TrainingService`; the CQRS article splits that shape into one handler per command/query. Keep the multi-method service when the methods share one clear application concept, and split commands and queries when read and write paths have different dependencies or models.

The CQRS article bundles handlers behind an application struct:

```go
type Application struct {
    Commands Commands
    Queries  Queries
}

type Commands struct {
    CancelTraining CancelTrainingHandler
}

type Queries struct {
    AvailableHours AvailableHoursHandler
}
```

For cross-cutting command logging, use the article's deferred wrapper pattern: `LogCommandExecution` records the start, defers logging of the final error/result, and then calls the inner handler.

## Interfaces

Define interfaces where they are consumed, not where implementations live. Put repository interfaces with the aggregate/domain when they express aggregate persistence. Put external service ports in the app package that orchestrates them.

Good:

```go
type OrderRepository interface {
    Get(ctx context.Context, id OrderID) (*Order, error)
    Save(ctx context.Context, order *Order) error
}
```

Avoid broad ports:

```go
type Database interface {
    Query(ctx context.Context, sql string, args ...any) (*Rows, error)
    Exec(ctx context.Context, sql string, args ...any) error
}
```

The interface should describe the use case need, not the mechanics of a dependency. Split interfaces that mix unrelated workflows.

## Transport Layer

Handlers should:

- Decode and validate transport shape.
- Convert request data into command/query types.
- Call one application use case.
- Map known errors to protocol responses.
- Keep framework types (`gin.Context`, `echo.Context`, generated request objects) at the edge.

Handlers should not:

- Open transactions.
- Build SQL queries.
- Mutate aggregates directly when an application service exists.
- Publish domain events directly when the app layer owns the workflow.
- Depend on concrete database clients unless the service is intentionally tiny.

## Cross-Context Calls

Prefer one of these:

- **Direct port** for synchronous in-process access with a stable request/response contract.
- **Domain/application event** for asynchronous reactions.
- **Shared kernel/common package** only for stable cross-cutting primitives, not business shortcuts.

Avoid importing another bounded context's domain package directly unless the repo explicitly allows it.

## Dependency Injection

Prefer explicit constructors and struct fields. Use code generation or DI frameworks only if the project already uses them.

```go
func NewServer(log *logrus.Entry, orders PlaceOrderHandler) *http.Server {
    mux := http.NewServeMux()
    registerOrderRoutes(mux, log, orders)
    return &http.Server{Handler: mux}
}
```

Keep wiring centralized so business packages do not import infrastructure packages just to construct themselves. Composition code can be large in real systems; keep it grouped by context and covered by component tests.

## Boundary Checks

- Use `go list`, `rg`, or local architecture linters to inspect import direction.
- Search ports for repository/database imports.
- Search domain for framework, SQL, broker, cloud, process, and telemetry imports.
- Search app for concrete adapter imports unless the repo intentionally wires there.
- Check whether commands and queries are named entry points, not anonymous service methods.
- Check whether component tests cover new wiring.

## Examples

Annotated reference implementations live in `examples/`. They draw from the Training/Trainer domain in the Three Dots Labs [Introducing Clean Architecture](https://threedots.tech/post/introducing-clean-architecture/) and [Basic CQRS in Go](https://threedots.tech/post/basic-cqrs-in-go/) posts:

- [`examples/command_handler.go`](examples/command_handler.go) — CQRS-post-style `CancelTrainingHandler` with three narrow interfaces (`trainingRepository`, `userService`, `trainerService`) defined in the handler package; shows nil-check constructor panics and the UpdateFn-style repo call.
- [`examples/query_handler.go`](examples/query_handler.go) — `AvailableHoursHandler` reading through `AvailableHoursReadModel` and returning flat `Date` DTOs, with notes on what CQRS does and does not require.

## Done Criteria

- A request path has one visible route from handler to use case to domain to adapter.
- Package imports show inward dependency direction.
- Infrastructure can be swapped or faked without editing domain code.
- Names describe workflows and domain concepts, not generic layers.
