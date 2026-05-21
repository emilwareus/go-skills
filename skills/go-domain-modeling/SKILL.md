---
name: go-domain-modeling
description: Model business domains in Go with DDD-oriented entities, value objects, aggregates, invariants, domain events, and domain services. Use for Go tasks involving business rules, aggregate boundaries, domain packages, ubiquitous language, refactoring anemic CRUD models, or applying Three Dots Labs-style domain-first design without coupling domain code to frameworks, databases, HTTP, queues, filesystems, process execution, or telemetry.
---

# Go Domain Modeling

Use this when a Go change touches business rules or domain package boundaries. Read local architecture docs before moving code. Keep domain packages independent of IO unless the repo documents an exception.

## Core Rules

- Start from the business language in product docs, tests, tickets, and existing code before inventing names.
- Keep domain packages free of HTTP, SQL, ORM, message broker, filesystem, process execution, CLI, framework, and telemetry imports unless the repo explicitly documents that exception.
- Put invariants next to the data they protect. Prefer constructors and methods that cannot create invalid state.
- Keep fields unexported when arbitrary mutation can break invariants.
- Choose aggregate boundaries around immediate consistency, not table shape or API response shape.
- Use domain events for facts that already happened, not commands to another component.
- Avoid generic `models`, `utils`, and `helpers` packages. Name packages after domain concepts or application responsibilities.

## Modeling Workflow

1. Identify the workflow the user is changing.
2. Extract nouns, decisions, state transitions, and invariants from code, tests, docs, and tickets.
3. Choose aggregate roots by asking what must be consistent immediately after one command.
4. Define value objects for validated concepts such as money, email, IDs, date ranges, status, and quantities.
5. Move behavior from handlers/services into domain methods when it enforces domain rules.
6. Keep orchestration, IO, retries, authorization, transaction management, and observability outside the domain.
7. Add black-box tests that describe business behavior without requiring a database, broker, server, provider, or framework.

## Package Shape

Prefer feature/domain packages over layer packages when the business area is small:

```text
internal/payments/
  invoice.go
  invoice_test.go
  service.go
  repository.go
```

Use subpackages when infrastructure or adapters would obscure the domain:

```text
internal/payments/
  domain/
  app/
  adapters/postgres/
  ports/httpapi/
  service/
```

Start with the repo's current layout. Split packages only when one package now mixes domain code with adapters, transports, or unrelated workflows.

## Entities And Value Objects

Use value objects for concepts with validation and behavior:

```go
type Email string

func NewEmail(value string) (Email, error) {
    value = strings.TrimSpace(strings.ToLower(value))
    if value == "" || !strings.Contains(value, "@") {
        return "", ErrInvalidEmail
    }
    return Email(value), nil
}
```

Use entities when identity matters across state changes. Methods should be named as business actions, not technical setters:

```go
type Invoice struct {
    id     InvoiceID
    status InvoiceStatus
    lines  []Line
}

func (i *Invoice) Approve(now time.Time) error {
    if i.status != InvoiceStatusDraft {
        return ErrInvoiceNotDraft
    }
    if len(i.lines) == 0 {
        return ErrInvoiceHasNoLines
    }
    i.status = InvoiceStatusApproved
    return nil
}
```

Prefer explicit rehydration for persisted state when a repository needs to restore fields that normal constructors should not expose:

```go
func RehydrateInvoice(id InvoiceID, status InvoiceStatus, lines []Line) (*Invoice, error) {
    inv := &Invoice{id: id, status: status, lines: append([]Line(nil), lines...)}
    if err := inv.validate(); err != nil {
        return nil, err
    }
    return inv, nil
}
```

Name this clearly so application code does not use it as an ordinary constructor.

## Aggregate Boundaries

Choose aggregates by asking:

- What invariant must hold immediately after one command?
- Which object owns that invariant?
- Can related state be eventually consistent instead?
- Will loading this aggregate require unbounded data?
- Does the repository need one transaction/lock to save this aggregate?

Avoid aggregates that mirror an entire database relationship graph. If one command needs many aggregates, classify the case before coding: application workflow, eventual-consistency flow, or unclear consistency rule.

## Domain Services

Use a domain service only when behavior belongs to the domain but not naturally to one entity or value object. If the service fetches from a database, calls a provider, emits logs, opens transactions, or uses HTTP, it is not a pure domain service.

Keep domain services pure:

```go
type PricingPolicy struct{}

func (p PricingPolicy) Price(order Order, customer Customer) (Money, error) {
    // domain decision, no SQL or HTTP
}
```

If external data is required, pass already-loaded domain data into the service. Let the application layer fetch it.

## Anti-Patterns

- Putting validation only in HTTP handlers while domain constructors accept invalid state.
- Returning ORM models from domain methods.
- Passing `*http.Request`, `*gin.Context`, `*sql.Tx`, `*gorm.DB`, `multipart.FileHeader`, provider clients, or tracers into domain methods.
- Creating generic repository abstractions before there are multiple implementations.
- Encoding workflow state as loose strings used across many packages.
- Using domain events to hide synchronous dependencies that should be explicit.
- Moving IO into domain code to make a test easier to write.

## Review Checklist

- Search changed domain packages for imports such as `net/http`, `database/sql`, `gorm`, `gin`, `os/exec`, broker clients, cloud SDKs, and telemetry vendors.
- Check whether exported constructors can create invalid states.
- Check whether behavior was placed in handlers or app services only because dependencies were easier to access there.
- Check whether new events are named as past-tense facts from this domain.
- Check whether tests exercise exported behavior from the outside.

## Done Criteria

- Domain tests run without network, database, broker, provider, filesystem, process, or framework setup.
- Constructors and methods enforce invariants that comments or handlers used to describe.
- Application services orchestrate IO; domain objects enforce rules.
- Package names use the team's business language.
