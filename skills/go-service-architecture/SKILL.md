---
name: go-service-architecture
description: Structure Go services with clear package boundaries, dependency direction, ports, application use cases, adapters, direct ports, events, and dependency injection. Use for Go backend architecture, Clean Architecture, Hexagonal Architecture, DDD service layout, CQRS command/query surfaces, refactoring tangled packages, introducing ports/adapters, or deciding where HTTP, gRPC, CLI, queues, config, domain, app, adapter, and service wiring code belong.
---

# Go Service Architecture

Use this when package boundaries, dependency direction, or service wiring are part of a Go change. Preserve the repo's existing layout and make dependencies point toward business behavior. Keep the guidance practical: where code belongs, how dependencies flow, and which architectural shortcuts to catch.

## Dependency Direction

Use this dependency direction:

```text
ports/transports -> application/use cases -> domain
adapters/infrastructure -> application/use cases -> domain
service/cmd composition wires concrete implementations
```

Use these layer responsibilities:

- **Domain**: business rules and domain types.
- **Application**: commands, queries, and use-case orchestration.
- **Ports**: HTTP, gRPC, CLI, queue, and message entry points.
- **Adapters**: database, external-service, Pub/Sub, filesystem, and infrastructure implementations.

Domain packages stay focused on business rules. Application packages orchestrate IO through narrow interfaces. Composition code wires concrete implementations.

## Workflow Placement

1. Inspect the current package layout.
2. Identify the workflow: command, query, background job, integration event, direct port, or migration.
3. Put transport parsing and response mapping at the edge.
4. Put orchestration in an application service, command handler, query handler, or use case.
5. Put business decisions in domain types.
6. Put SQL, external APIs, queues, clocks, random IDs, file systems, and process execution behind narrow interfaces.
7. Wire concrete dependencies in `main`, `cmd`, `service`, or the repo's established composition package.
8. Cover new wiring with component tests when behavior crosses package boundaries.

## Package Layout

Small services can keep one cohesive package per domain area:

```text
cmd/api/main.go
internal/orders/
  order.go
  service.go
  repository.go
  http.go
  postgres.go
```

Split packages when responsibilities are already distinct:

```text
internal/orders/
  domain/
  app/
  adapters/postgres/
  ports/httpapi/
  service/
```

Add folders when they clarify ownership, import direction, or adapter boundaries.

## Application Handlers

Use one command/query handler per workflow when dependencies or read/write models differ:

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

Use a cohesive multi-method application service when the methods share one clear application concept and the repo already follows that style. Split toward command/query handlers when read and write paths have different dependencies, models, or authorization rules.

Bundle CQRS handlers behind an application struct:

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

Use decorators for cross-cutting behavior such as command logging, metrics, authorization, and retries. A command logging decorator should record the start, defer final logging, and include the returned error/result after the inner handler finishes.

## Interfaces

Define interfaces where they are consumed:

```go
type trainingRepository interface {
    CancelTraining(ctx context.Context, trainingUUID string, updateFn func(context.Context, *Training) error) error
}

type trainerService interface {
    CancelTraining(ctx context.Context, trainingTime time.Time) error
}
```

Interfaces should describe the use-case need. Keep them narrow enough to fake in tests and broad enough to avoid leaking implementation mechanics.

## Transport Layer

Handlers should:

- Decode and validate transport shape.
- Convert request data into command/query types.
- Call one application use case.
- Map known errors to protocol responses.
- Keep framework types at the edge.

Handlers should not open transactions, build SQL queries, mutate aggregates directly when an application use case exists, publish workflow events directly when the app layer owns the command, or depend on concrete database clients unless the service is intentionally tiny.

## Cross-Context Calls

Use one of these integration styles:

- **Direct port** for synchronous in-process access with a stable request/response contract.
- **Domain/application event** for asynchronous reactions.
- **Shared kernel/common package** for stable cross-cutting primitives.

Avoid importing another context's domain package directly unless the repo explicitly accepts that coupling.

## Dependency Injection

Prefer explicit constructors and struct fields:

```go
func NewServer(log *logrus.Entry, orders PlaceOrderHandler) *http.Server {
    mux := http.NewServeMux()
    registerOrderRoutes(mux, log, orders)
    return &http.Server{Handler: mux}
}
```

Keep wiring centralized so business packages stay independent from concrete infrastructure packages.

## Anti-Patterns

- Domain packages importing HTTP, gRPC, SQL, queues, config, logging libraries, telemetry vendors, filesystem/process APIs, or service lifecycle code.
- Application packages importing concrete adapters when composition should inject narrow ports.
- Broad interfaces named after mechanics, such as database-shaped ports, instead of use-case needs.
- Generic manager/service objects with unrelated methods and unclear workflow names.
- Route handlers or message handlers that perform orchestration, transactions, SQL, and domain mutation inline.
- Copying a large clean-architecture template into a small service before responsibilities need those packages.
- Cross-context domain imports used as shortcuts around a direct port or event contract.

## Boundary Checks

- Use `go list`, `rg`, or local architecture linters to inspect import direction.
- Search ports for repository/database imports.
- Search domain for framework, SQL, broker, cloud, process, and telemetry imports.
- Search app for concrete adapter imports unless the repo intentionally wires there.
- Check whether commands and queries are named entry points.
- Check whether component tests cover new wiring.

## Examples

- [`examples/command_handler.go`](examples/command_handler.go) - `CancelTrainingHandler` with narrow local interfaces and an UpdateFn-style repository call.
- [`examples/query_handler.go`](examples/query_handler.go) - `AvailableHoursHandler` reading through `AvailableHoursReadModel` and returning flat read DTOs.

## Done Criteria

- A request path has one visible route from handler to use case to domain to adapter.
- Package imports show inward dependency direction.
- Infrastructure can be swapped or faked without editing domain code.
- Names describe workflows and domain concepts.
