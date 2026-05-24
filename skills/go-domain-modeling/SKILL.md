---
name: go-domain-modeling
description: Model Go business rules with DDD-Lite — entities with unexported fields, state-transition methods, sentinel errors. Use for refactoring anemic CRUD handlers into aggregates whose methods cannot leave the aggregate in an invalid state.
---

# Go Domain Modeling

Scope: the "DDD-Lite" patterns from Three Dots Labs' [Introduction to DDD Lite](https://threedots.tech/post/ddd-lite-in-go-introduction/) — refactoring procedural handler code into an aggregate that owns its invariants. No coupling to the wider DDD vocabulary (bounded contexts, ubiquitous language, event sourcing) beyond what that article uses.

## The refactor this skill is about

The article opens with a procedural handler:

```go
func (h ScheduleTrainingHandler) Handle(ctx context.Context, hour time.Time) error {
    if hour.Before(time.Now()) { return errors.New("cannot schedule in the past") }
    // ... more guards ...
    return h.repo.Update(ctx, hour, func(h *Hour) (*Hour, error) {
        if h.Availability != "available" { return nil, errors.New("hour not available") }
        h.Availability = "training_scheduled"
        return h, nil
    })
}
```

…and moves every state-transition rule onto the `Hour` aggregate so the handler shrinks to one method call. This skill is about doing that refactor.

## Aggregate rules

- Unexported fields. No path to mutation that bypasses methods.
- One method per business state transition, named for the business action (`ScheduleTraining`, `CancelTraining`, `MakeAvailable`).
- Each transition method validates that the transition is currently legal and returns a sentinel error if not.
- Queries (`IsAvailable`, `HasTrainingScheduled`) are read-only and side-effect free.
- Sentinel errors are package-level `var`s declared once (`ErrHourNotAvailable`), compared with `errors.Is`.

## Constructor rules

- Named constructors per initial state (`NewAvailableHour`, `NewNotAvailableHour`) — not one constructor with an `Availability` parameter the caller has to remember the meaning of.
- All validation lives inside the constructor. A returned `*Hour` is always safe to use.

## Aggregate boundaries

Group data that must be transactionally consistent into one aggregate. If `Hour` and `Availability` must be updated together under one lock, they belong to the same aggregate — even if they live in different database tables.

The repository for an aggregate exposes an `UpdateFn`-style method:

```go
type Repository interface {
    UpdateHour(ctx context.Context, hourTime time.Time, updateFn func(*Hour) (*Hour, error)) error
}
```

The repository owns transactions and locks; the closure makes domain decisions. See the [`go-persistence-transactions`](../go-persistence-transactions/SKILL.md) skill for the persistence side.

## Anti-patterns

- Public fields on aggregates ("anemic model"). The rule about *when* you can transition becomes the responsibility of every caller.
- Constructors that accept invalid state and rely on a separate `Validate()` call.
- State transitions implemented as `SetAvailability(string)` rather than `ScheduleTraining()`. The setter is a wrong-shaped API: it lets a caller pass `"training_scheduled"` without checking the current state.
- Domain code that imports `net/http`, `database/sql`, `gorm`, `gin`, broker clients, cloud SDKs, or telemetry vendors.
- Returning ORM models or DTOs from domain methods.

## Examples

- [`examples/hour_aggregate.go`](examples/hour_aggregate.go) — the `Hour` aggregate from the DDD Lite article: unexported `hour`/`availability` fields, named constructors, `ScheduleTraining`/`CancelTraining`/`MakeAvailable` transition methods, sentinel errors, `IsAvailable`/`HasTrainingScheduled` queries.

## Done criteria

- The handler that motivated the change is one method call into an aggregate method.
- Every state transition rule lives on the aggregate, not in the handler.
- The aggregate's tests run without a database, broker, or framework.
