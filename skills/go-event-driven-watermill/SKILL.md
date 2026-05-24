---
name: go-event-driven-watermill
description: Build Go event-driven workflows with Watermill, Pub/Sub, CQRS command/event buses, routers, middleware, outbox/forwarder patterns, idempotent consumers, and component tests. Use for Go tasks involving domain events, async handlers, message routing, Ack/Nack, retries, dead letters, event contracts, transactional event publishing, or replacing synchronous cross-service calls with events.
---

# Go Event-Driven Watermill

Use this when a Go workflow needs asynchronous messaging, Watermill routing, or reliable event publication. Keep broker details in adapters and keep business behavior in domain/application handlers. The useful lessons are event naming, durable publication, thin router callbacks, idempotent consumers, and bounded tests.

## Event Names

Publish events as facts that already happened in the publishing domain:

```text
InvoiceApproved
PointsUsed
TrainingRescheduled
LotteryConcluded
```

Name events for the fact the publishing domain knows. `PointsUsed` is a user-domain fact. `PointsUsedForDiscount` leaks the orders use case into the user event name and should be reshaped before it becomes a contract.

Event payloads should include the IDs and metadata consumers need:

- Aggregate ID.
- Tenant or organization ID when the system is multi-tenant.
- Occurred-at timestamp.
- Correlation or causation ID.
- Additive fields or explicit versions for schema evolution.

## Watermill Basics

Keep these Watermill concepts at the adapter/composition boundary:

- `message.Message`: UUID, metadata, payload, context, and Ack/Nack state.
- `message.Publisher`: publishes messages to a topic.
- `message.Subscriber`: subscribes to a topic.
- `message.Router`: connects subscribers, handlers, and optional publishers.
- Middleware: correlation, retry, recoverer, logging, metrics, tracing, and dead-letter behavior when the project uses them.
- CQRS helpers: typed command bus, event bus, command processors, and event processors when that package is already in use.

Treat messages as immutable after publishing. Use metadata for correlation and bounded routing attributes.

## Application Boundary

Watermill handlers should translate messages into application commands or queries:

```go
type OnPointsUsed struct {
    addDiscount AddDiscountHandler
}

func (h OnPointsUsed) Handle(ctx context.Context, event *PointsUsed) error {
    return h.addDiscount.Handle(ctx, AddDiscount{
        UserID:   event.UserID,
        Discount: event.Points,
    })
}
```

Application services can depend on narrow publisher interfaces:

```go
type EventPublisher interface {
    Publish(ctx context.Context, event any) error
}
```

Keep marshaling, topic names, broker config, router setup, and Ack/Nack behavior in adapters or service composition.

## Router Workflow

Use one explicit router setup path:

1. Create a logger adapter that matches the repo logging standard.
2. Create concrete publisher and subscriber adapters.
3. Create a `message.Router`.
4. Add signal/shutdown handling for long-lived routers.
5. Add middleware in a deliberate order: correlation, retry, recoverer, observability.
6. Register handlers with stable unique names.
7. Run the router with a lifecycle context.

## Outbox Pattern

Use an outbox when one command must persist state and publish events reliably:

1. Domain/application code decides which event facts occurred.
2. The repository saves aggregate changes and outgoing event rows/messages in one database transaction.
3. A forwarder publishes committed messages to the broker.
4. Consumers handle duplicate delivery idempotently.

Use Watermill SQL Pub/Sub plus Forwarder when the project already uses Watermill components. A hand-rolled outbox needs the same guarantees: committed rows are eventually forwarded, rolled-back rows are never published, and duplicate delivery is expected.

The consistency tradeoff is waiting, not accepting silent loss. If the broker or downstream service is unavailable, the event waits durably until it can be forwarded.

The UpdateFn outbox shape keeps publication coupled to the database commit:

```go
type OutboxUserRepository interface {
    UpdateByID(ctx context.Context, userID int, updateFn func(*User) (bool, []any, error)) error
}
```

## Ordering And Consumer Groups

When order matters:

- Define the ordering key, such as aggregate ID or tenant plus aggregate ID.
- Choose broker, topic, and partition settings that preserve order for that key.
- Keep one ordered stream per consistency need.
- Make handlers tolerate replay after partial failure.

## Error Handling

- Return nil after the message is safely handled or intentionally ignored.
- Return an error for transient failures so retry middleware or broker redelivery can run.
- Route permanently invalid messages to a dead-letter or quarantine path when the infrastructure supports it.
- Log final failures with message UUID, event type, topic, correlation ID, aggregate ID, and error.
- Ack only after durable side effects are complete.

## Anti-Patterns

- Event names that describe a downstream command, service, or side effect instead of the publishing-domain fact.
- Synchronous cross-service calls after a local state change when the workflow assumes atomic behavior across services.
- Persisting state and then publishing to a broker with no outbox or recovery path.
- Publishing first and then writing state when consumers may observe facts that never commit.
- Putting business decisions in Watermill callbacks instead of application command/query handlers.
- Acking before durable side effects are complete.
- Consumers that are not idempotent or cannot handle duplicate delivery.
- Fixed sleeps in message tests instead of bounded eventual assertions.

## Testing

Use the smallest scope that covers the risk:

- Unit test domain/application behavior without Watermill.
- Component test "message in -> observable state out" with the real router and mocked external providers.
- Integration test broker behavior such as Ack/Nack, ordering, retries, SQL Pub/Sub, and forwarder behavior.
- Filter consumed events by unique ID or correlation metadata.
- Use bounded eventual assertions.
- Test duplicate delivery.

## Examples

- [`examples/handler_returns_events.go`](examples/handler_returns_events.go) - command handler returning domain events from the UpdateFn closure, plus a consumer that maps `PointsUsed` to an orders command.
- [`examples/outbox/repository.go`](examples/outbox/repository.go) - outbox-aware `UpdateByID` that saves aggregate state and event rows in one transaction.
- [`examples/outbox/schema.sql`](examples/outbox/schema.sql) - outbox table with a pending-row index.
- [`examples/outbox/forwarder.go`](examples/outbox/forwarder.go) - forwarder using `SELECT ... FOR UPDATE SKIP LOCKED` for safe multi-replica draining.

## Done Criteria

- Event names are facts from the publishing domain.
- Business behavior lives in domain/application handlers.
- Publisher/subscriber details are isolated in adapters or service wiring.
- Database-plus-event workflows use an outbox.
- Consumers are idempotent and tested for duplicate delivery.
