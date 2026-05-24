---
name: go-code-quality-patterns
description: Improve Go business code with explicit models, safe enums, deliberate duplication, cohesive decorators, low framework coupling, and small library choices. Use for Go refactors involving DRY decisions, shared DTOs, primitive obsession, overengineering, web-app anti-patterns, command decorators, or package cohesion.
---

# Go Code Quality Patterns

Use this when code is technically working but hard to change safely. Prefer obvious, explicit Go over clever abstractions. Keep business behavior easy to test without HTTP, database, framework, logging, or metrics setup.

## Deliberate Duplication

Duplicate data shapes when the callers change for different reasons:

- HTTP response DTO.
- Database/storage model.
- Event payload.
- Domain aggregate or value object.
- gRPC/OpenAPI generated type.

Use a shared struct only when the same concept must change together across those boundaries. DRY is safer for behavior than for boundary data. Map between shapes explicitly at the boundary.

## Valid State In Memory

Model important business values with constructors and private fields:

```go
type UserID struct {
    id string
}

func NewUserID(id string) (UserID, error) {
    if id == "" {
        return UserID{}, ErrEmptyID
    }
    return UserID{id: id}, nil
}
```

Use this for values where a raw string/int hides real rules: role, user ID, training UUID, date range, points amount, currency, email, or tenant ID. Check the zero value at boundaries when the type can be constructed as `UserID{}`.

## Safe Enums

For business logic, prefer struct-backed enums with unexported slug fields:

```go
type Role struct {
    slug string
}

func (r Role) String() string { return r.slug }
```

Keep valid values in the enum package and parse external strings through a constructor such as `RoleFromString`. Use string slugs for APIs, logs, database rows, and error codes when the value is part of a contract.

Use `iota` only when numeric values are private implementation details and never cross a boundary.

## Cohesive Handlers

Keep the command/query handler focused on the use case. Move cross-cutting behavior to decorators:

- Authorization.
- Logging.
- Metrics.
- Tracing.
- Retry or timeout policy.

Decorators wrap a handler and call the base handler after doing their own job. Choose decorators in constructors/composition so tests can instantiate the core handler without logging, metrics, auth, or framework setup.

Use generic command/query decorators when the repo already has a common handler interface. Do not introduce generics just to make one handler shorter.

## Framework And Library Choice

Prefer libraries that do one job and compose with the standard library:

- HTTP router over full-stack web framework.
- OpenAPI/gRPC code generation for boundary contracts.
- Database driver or query builder that does not force domain types to become storage models.
- Message library isolated at adapter/composition boundaries.

Pick a framework only when its conventions are an explicit tradeoff the team accepts for the product's lifetime. Keep domain and application code independent from the framework either way.

Choose libraries for API quality, maintenance, integration with the standard library, and fit for the problem. Do not choose only by benchmark, GitHub stars, or a long feature list. For trivial behavior that the standard library handles cleanly, prefer the standard library.

## Package And Directory Decisions

Let coupling drive package boundaries:

- Group code that changes together.
- Separate HTTP/database/message code from business behavior when those concerns already pull in different dependencies.
- Define small interfaces where consumed.
- Keep generated API types out of domain packages.
- Start with fewer directories for tiny services; split when import direction or ownership becomes unclear.

Directory layout is a convention. The meaningful design is how packages and structs reference each other.

## Complexity Review

Before adding an abstraction, check:

- Does it remove a real dependency direction problem?
- Does it let tests focus on behavior?
- Does it make a business rule easier to read?
- Does the same variation already exist in at least two places?
- Can a simple explicit mapping or constructor solve it?

Before deleting "boilerplate", check whether that duplicated code protects two boundaries that should change independently.

## Anti-Patterns

- One struct reused for API, database, event, and domain state.
- Raw primitives for values with business rules.
- Validation that relies on every caller remembering to call `Validate`.
- Command handlers mixed with authorization, logging, metrics, persistence details, and transport parsing.
- Package splits designed before the first real dependency pressure appears.
- Full framework adoption for glue that a small library already handles.
- Generic helpers that hide business names or make call sites less explicit.

## Examples

- [`examples/role_enum.go`](examples/role_enum.go) - struct-backed enum with slug parsing and zero-value check.
- [`examples/generic_decorator.go`](examples/generic_decorator.go) - command handler plus generic logging decorator that keeps the base handler free of logging.

## References

- [When to avoid DRY in Go](https://threedots.tech/post/things-to-know-about-dry/)
- [Safer Enums in Go](https://threedots.tech/post/safer-enums-in-go/)
- [Increasing Cohesion in Go with Generic Decorators](https://threedots.tech/post/increasing-cohesion-in-go-with-generic-decorators/)
- [Common Anti-Patterns in Go Web Applications](https://threedots.tech/post/common-anti-patterns-in-go-web-applications/)
- [The Best Go framework: no framework?](https://threedots.tech/post/best-go-framework/)
- [The Go libraries that never failed us](https://threedots.tech/post/list-of-recommended-libraries/)
- [The Over-Engineering Pendulum](https://threedots.tech/post/the-over-engineering-pendulum/)

## Done Criteria

- Boundary DTOs, storage models, events, and domain objects are separated when they change independently.
- Important values cannot be invalid without crossing an explicit parse/constructor boundary.
- Cross-cutting behavior is composed around use-case handlers.
- Framework/library choices do not leak into domain and application code.
- New abstractions remove real coupling or test friction.
