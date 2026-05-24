---
name: go-event-driven-watermill
description: Build Go event-driven workflows with Watermill, Pub/Sub, CQRS command/event buses, routers, middleware, outbox/forwarder patterns, idempotent consumers, and component tests. Use for Go tasks involving Three Dots Labs Watermill examples, domain events, async handlers, message routing, Ack/Nack, retries, dead letters, event contracts, transactional event publishing, or replacing synchronous cross-service calls with events.
---

# Go Event-Driven Watermill

Use this when a Go workflow needs asynchronous messaging, Watermill routing, or reliable event publication. Keep Watermill and broker details outside pure domain code; inject small publisher, subscriber, or application interfaces at the boundary that needs them.

## Message Design

- Publish events as facts that already happened in the publishing domain.
- Do not name events as commands to downstream services.
- Evolve event payloads with additive fields or explicit versioning.
- Include aggregate IDs, tenant/org IDs, occurred-at timestamps, and correlation IDs when consumers need them.
- Avoid leaking private database or provider details into event contracts.
- Make consumers idempotent; duplicate delivery is expected.

Good event names:

```text
InvoiceApproved
PointsUsed
TrainingRescheduled
LotteryConcluded
```

Poor event names:

```text
SendEmailNow
ApplyDiscountInOrdersService
CallBillingWebhook
PointsUsedForDiscount
```

`PointsUsedForDiscount` is the article's own case study: it is an event from the users service, but the name exposes the orders service's discount use case. `PointsUsed` names the publishing-domain fact more cleanly.

## Watermill Basics

Core abstractions:

- `message.Message`: UUID, metadata, payload, context, Ack/Nack state.
- `message.Publisher`: publishes messages to a topic.
- `message.Subscriber`: subscribes to a topic.
- `message.Router`: routes subscribed messages to handlers and optional publishers.
- Middleware: retry, recoverer, correlation, poison/dead-letter, metrics, tracing, logging.
- CQRS package: typed command bus, event bus, command processors, event processors.

Treat messages as immutable after publish. Use metadata for correlation and bounded routing attributes.

## Router Workflow

1. Create a logger adapter that matches the repo logging standard.
2. Create concrete publisher/subscriber adapters.
3. Create a `message.Router`.
4. Add signal/shutdown plugin if the router is long-lived.
5. Add middleware in intentional order: correlation, retry, recoverer, observability.
6. Register handlers with unique names.
7. Run the router with a lifecycle context.

Middleware order matters. A common order is correlation first, retry around handler errors, recoverer innermost so panics become errors that retry can handle.

## Application Boundary

Do not put business rules in Watermill callbacks. Translate message payloads into application commands or queries:

```go
type OnInvoiceApproved struct {
    sendReceipt SendReceiptHandler
}

func (h OnInvoiceApproved) Handle(ctx context.Context, event *InvoiceApproved) error {
    return h.sendReceipt.Handle(ctx, SendReceipt{
        InvoiceID: event.InvoiceID,
        CustomerID: event.CustomerID,
    })
}
```

Application services depend on narrow event publisher interfaces:

```go
type EventPublisher interface {
    Publish(ctx context.Context, event any) error
}
```

Keep Watermill marshaling, topic naming, broker config, and router setup in adapters/service composition.

## Outbox Pattern

Use the outbox pattern when a command must both persist state and publish an event reliably:

1. Application/domain decides which event facts occurred.
2. Repository/transactor saves aggregate changes and outgoing event rows/messages in one DB transaction.
3. Commit.
4. A forwarder publishes stored messages to the broker.
5. Consumers process idempotently.

Avoid:

- Persisting data and then publishing directly to a broker with no recovery path.
- Publishing first and then writing data.
- Holding a DB transaction while calling an external broker directly unless using a SQL Pub/Sub/outbox writer inside that transaction.

The article's reference implementation uses Watermill SQL Pub/Sub plus Watermill Forwarder out of the box. The `examples/outbox/*` files in this skill are hand-rolled illustrations of the same mechanism, not a replacement recommendation.

The thesis is not "accept inconsistency between services." It is "accept waiting": when the broker or downstream service is unavailable, the event waits in durable storage until it can be forwarded.

## Ordering And Consumer Groups

When order matters:

- Define the ordering key, such as aggregate ID or tenant+aggregate ID.
- Choose broker/topic/partition config that preserves that order.
- Keep one ordered stream per consistency need, not one global bottleneck.
- Make handlers tolerate replay after partial failure.

For CQRS read models, handler groups can share subscriptions while routing by event type.

## Error Handling

- Return nil only after the message is safely handled or intentionally ignored.
- Return an error for transient failures so middleware/broker can retry.
- Classify permanently invalid messages and route to a dead-letter or quarantine path when available.
- Log final failures with message UUID, event type, topic, correlation ID, aggregate ID, and error.
- Do not Ack before durable side effects unless the workflow explicitly accepts loss.

## Testing

Use the smallest scope that covers the risk:

- Unit test domain/application behavior without Watermill.
- Component test "message in -> observable state out" with the real router and mocked external providers.
- Integration test broker-specific behavior such as Ack/Nack, ordering, retries, SQL Pub/Sub, and forwarder.
- Use unique IDs and correlation metadata so parallel tests filter their own events.
- Use bounded eventual assertions instead of fixed sleeps.
- Test duplicate delivery.

## Examples

Annotated reference implementations live in `examples/`. `handler_returns_events.go` mirrors the Three Dots Labs [Distributed Transactions in Go](https://threedots.tech/post/distributed-transactions-in-go/) handler evolution — same `User` aggregate, `UsePointsAsDiscount` command, and `PointsUsedForDiscount` event. The `outbox/` files are hand-rolled illustrations of the Watermill SQL Pub/Sub + Forwarder mechanism the article uses.

- [`examples/handler_returns_events.go`](examples/handler_returns_events.go) — the article's three-version evolution of the handler: distributed monolith → publish-after-commit → outbox via `UpdateByID(ctx, id, func(*User) (bool, []any, error))`. The distributed monolith version has two services but synchronous, cross-service consistency assumptions; the outbox version keeps local consistency and waits durably for delivery. Includes the consuming `OnPointsUsedForDiscountHandler` that maps the event to an `AddDiscount` command.
- [`examples/outbox/repository.go`](examples/outbox/repository.go) — the outbox-aware `UpdateByID` body: same load → mutate → save shape as the persistence skill's `update_fn.go`, with event INSERTs into the outbox in the same transaction.
- [`examples/outbox/schema.sql`](examples/outbox/schema.sql) — hand-rolled equivalent of Watermill SQL Pub/Sub's table, with a partial index on pending rows.
- [`examples/outbox/forwarder.go`](examples/outbox/forwarder.go) — hand-rolled equivalent of Watermill's Forwarder, using `SELECT ... FOR UPDATE SKIP LOCKED` for safe multi-replica draining.

## Done Criteria

- Event names are facts from the publishing domain.
- Business behavior lives in domain/application handlers, not Watermill router callbacks.
- Publisher/subscriber details are isolated in adapters or service wiring.
- DB-plus-event workflows use an outbox or a documented accepted-loss tradeoff.
- Consumers are idempotent and tested for duplicate delivery.
- Observability includes correlation IDs and bounded-cardinality labels.
