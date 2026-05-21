---
name: go-service-architecture
description: Structure Go services with clear package boundaries, dependency direction, ports, application use cases, adapters, direct ports, events, and dependency injection. Use for Go backend architecture, Clean Architecture, Hexagonal Architecture, DDD service layout, CQRS command/query surfaces, refactoring tangled packages, introducing ports/adapters, or deciding where HTTP, gRPC, CLI, queues, config, domain, app, adapter, and service wiring code belong.
---

# Go Service Architecture

Use this skill to keep Go services boring, explicit, and easy to change. Read local docs such as `ARCHITECTURE.md`, `CODE_PATTERNS.md`, `CLAUDE.md`, `AGENTS.md`, or `TESTING_GUIDELINES.md` before moving boundaries.

## Architecture Rule

Dependencies point inward:

```text
ports/transports -> app/use cases -> domain
adapters/infrastructure -> app/use cases -> domain
service/cmd composition wires everything
```

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

Split adapters when the package becomes hard to scan, has multiple transports/adapters, or needs mechanical import rules:

```text
internal/orders/
  domain/
  app/
  adapters/postgres/
  ports/httpapi/
  service/
```

Avoid copying a large template into a small service. Add folders when they reduce cognitive load or match local architecture.

## Application Services

Application services coordinate IO, authorization, transaction scope, idempotency, logging/tracing boundaries, and domain calls. They should be easy to read top to bottom:

```go
type PlaceOrderHandler struct {
    orders OrderRepository
    tx     Transactor
    clock  Clock
}

func (h PlaceOrderHandler) Handle(ctx context.Context, cmd PlaceOrder) error {
    return h.tx.WithinTx(ctx, func(ctx context.Context) error {
        order, err := NewOrder(cmd.CustomerID, cmd.Lines, h.clock.Now())
        if err != nil {
            return err
        }
        return h.orders.Save(ctx, order)
    })
}
```

Do not let application services become generic "manager" objects. Keep one method or handler per use case when workflows differ. Split commands and queries when read and write paths have different dependencies or models.

## Interfaces

Define interfaces where they are consumed, not where implementations live. Repository interfaces often belong with the aggregate/domain when they express aggregate persistence; external service ports often belong in the app package that orchestrates them.

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

The interface should describe the business need of the use case, not the mechanics of a dependency. If an interface has many unrelated methods, split it by use case.

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
func NewServer(log *slog.Logger, orders PlaceOrderHandler) *http.Server {
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
- Check whether commands and queries expose a clear application surface.
- Check whether component tests cover new wiring.

## Done Criteria

- A new developer can trace a request from handler to use case to domain to adapter.
- Package imports show inward dependency direction.
- Infrastructure can be swapped or faked without editing domain code.
- Names describe workflows and domain concepts, not generic layers.
