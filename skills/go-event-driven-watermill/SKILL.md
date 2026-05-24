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

Use messages for transport concerns and events for domain facts. A Watermill message carries UUID, metadata, payload, context, and Ack/Nack state. The event is the decoded payload your application understands.

## Distributed Consistency

Use events when a workflow crosses service boundaries and immediate consistency is not required by the business. This is not accepting inconsistency permanently; it is accepting that consistency may arrive after the publisher commits.

Before adding a distributed transaction, check:

- Are the service boundaries wrong?
- Can the workflow be owned by one aggregate or one service instead?
- Can the downstream effect be retried from an event?
- Can the user see a pending state while the system catches up?
- Does the business accept a delay for this specific operation?

When the answer is yes, publish an event from the local transaction and let the downstream service react asynchronously. When the answer is no, keep the operation inside one consistency boundary or redesign the split.

## Watermill Basics

Keep these Watermill concepts at the adapter/composition boundary:

- `message.Message`: UUID, metadata, payload, context, and Ack/Nack state.
- `message.Publisher`: publishes messages to a topic.
- `message.Subscriber`: subscribes to a topic.
- `message.Router`: connects subscribers, handlers, and optional publishers.
- Middleware: correlation, retry, recoverer, logging, metrics, tracing, and dead-letter behavior when the project uses them.
- CQRS helpers: typed command bus, event bus, command processors, and event processors when that package is already in use.

Treat messages as immutable after publishing. Use metadata for correlation and bounded routing attributes.

Watermill is a library, not a framework. Do not make domain/application packages import Watermill types just because the adapter uses Watermill.

## CQRS Event Bus And Processor

When the repo uses Watermill's `cqrs` package:

- Configure publisher/subscriber infrastructure in composition or adapters.
- Use the same marshaler and topic naming rules for event bus and event processor.
- Generate publish/subscribe topics from the event name only when that is the repo contract.
- Keep typed handlers focused on mapping the event to an application command.
- Register handlers with stable unique names.

The event processor is another transport entry point, like HTTP. It should decode, call an application handler, and return an error only when retry/redelivery should happen.

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

Use SQL, PostgreSQL, MySQL, or SQLite Pub/Sub when durable message rows are useful and adding a separate broker is not justified. SQL Pub/Sub is also a natural fit for outbox/forwarder flows because the publish operation can participate in the same database transaction.

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

The reference outbox shape publishes into SQL through a transaction-bound event bus and uses Watermill Forwarder to move committed messages to the real broker. This repo's `examples/outbox/*` keeps the SQL rows visible as a readable implementation sketch.

## Poison And Delayed Requeue

For handlers that may fail repeatedly:

- Use retry middleware for transient failures.
- Move permanently failing or repeatedly failing messages to a poison/dead-letter path.
- Keep tooling or operational queries for inspecting poison messages.
- Prefer delayed requeue when one bad message should not block the topic forever.
- Preserve message metadata needed for debugging: message UUID, event type, topic, correlation ID, aggregate ID, and error.

Do not hide infinite retry loops. Operators need visibility and a way to retry or discard poison messages intentionally.

## Durable Execution

For workflows that must survive process crashes, DNS/network outages, and handler interruptions:

- Persist validated input before processing it.
- Use a persistent Pub/Sub backend for durable input; avoid in-memory Go channels for production input that must survive crashes.
- Acknowledge the message only after durable side effects complete.
- Keep all state changes for one event handler inside one database transaction.
- If the handler publishes another event while changing state, use the outbox pattern so state and outgoing event commit together.
- Make the handler idempotent so duplicate delivery does not change the final state.
- Prove idempotency by duplicating messages in tests and asserting the same final state.
- Prove atomicity with controlled failure/cancellation tests around retry middleware.

Durability is a property you test. Treat "it usually works" as insufficient for event handlers that produce business-critical output.

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
