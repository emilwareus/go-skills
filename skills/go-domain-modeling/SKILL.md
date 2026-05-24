---
name: go-domain-modeling
description: Model Go business rules with aggregates, unexported fields, business-named state-transition methods, and sentinel errors. Use for refactoring procedural handlers into domain types whose methods keep state valid.
---

# Go Domain Modeling

Use this when a change adds or moves business rules. Put state-transition rules on domain types, keep domain state private, and make handlers call business-named methods. Keep the skill focused on the model you should build and the traps to catch during refactors.

## Aggregate Pattern

Use aggregates for state that must stay consistent together. The aggregate should make illegal state changes hard to express:

```go
type Hour struct {
    hour         time.Time
    availability Availability
}

func (h *Hour) ScheduleTraining() error {
    if !h.IsAvailable() {
        return ErrHourNotAvailable
    }
    h.availability = TrainingScheduled
    return nil
}
```

Apply these rules:

- Fields are unexported.
- State changes happen through methods named after business actions.
- Each state-transition method checks the current state before mutating.
- Read methods such as `IsAvailable` and `HasTrainingScheduled` are side-effect free.
- Stable transition failures use package-level sentinel errors.
- The handler calls one business method instead of repeating guard logic.

## Constructors

Use named constructors for valid initial states:

```go
func NewAvailableHour(hour time.Time) (*Hour, error) {
    if err := validateTime(hour); err != nil {
        return nil, err
    }
    return &Hour{hour: hour, availability: Available}, nil
}
```

Keep validation inside constructors. A returned aggregate should be ready to use without a separate validation step. Use rehydration constructors only for persistence boundaries, and keep them clearly named so callers understand they rebuild stored state rather than create new business state.

## Repository Boundary

Persist aggregates through repository methods that load, mutate, and save one consistency boundary:

```go
type Repository interface {
    UpdateHour(ctx context.Context, hourTime time.Time, updateFn func(*Hour) (*Hour, error)) error
}
```

The repository owns transactions and locks. The closure owns domain decisions. Group data into one aggregate when it must be updated under one consistency boundary, even if the database stores it in several tables.

## Package Boundary

Keep domain packages focused on business behavior:

- Domain code imports standard value packages such as `time` and `errors`.
- Transport, SQL, ORM, broker, cloud, filesystem, and telemetry dependencies stay outside the domain package.
- Domain methods return domain values and domain errors, not DTOs or ORM models.

## Anti-Patterns

- Public aggregate fields that let handlers or adapters bypass business methods.
- Setter-shaped APIs such as `SetAvailability(string)` when the real operation is `ScheduleTraining`, `CancelTraining`, or `MakeAvailable`.
- Constructors that return invalid values and require a later `Validate()` call.
- Validation only at the HTTP or CLI boundary while internal callers can still create invalid state.
- Repeated guard logic in handlers, repositories, or transports instead of one aggregate method.
- Domain packages importing frameworks, SQL clients, ORM models, broker clients, cloud SDKs, or telemetry vendors.
- Returning DTOs, ORM models, or transport response shapes from domain behavior.

## Examples

- [`examples/hour_aggregate.go`](examples/hour_aggregate.go) - `Hour` aggregate with unexported fields, named constructors, `ScheduleTraining`/`CancelTraining`/`MakeAvailable` transition methods, sentinel errors, and read-only query methods.

## Done Criteria

- Business state changes are expressed as aggregate methods.
- Handlers call aggregate methods instead of editing fields directly.
- Constructors return valid aggregate instances.
- Domain tests run without a database, broker, or framework.
