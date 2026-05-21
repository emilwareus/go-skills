---
name: go-errors-observability
description: Design Go error handling and observability with wrapped errors, sentinel/domain errors, typed error contracts, error classification, structured slog logging, metrics, tracing, context propagation, and HTTP/gRPC/message error mapping. Use for Go tasks involving custom error packages, slog/zap/logrus migration, OpenTelemetry, retries, incident debugging, Watermill handler failures, or making failures diagnosable without leaking internals.
---

# Go Errors And Observability

Use this skill to make Go failures understandable at the right layer. Read local error and observability docs first; many mature Go services use typed application errors, structured slog fields, and protocol-specific mappers.

## Error Principles

- Return errors; do not log and return the same error at every layer.
- Wrap errors with operation context using `%w`.
- Classify errors at boundaries, not deep inside infrastructure.
- Keep domain errors stable enough for callers to compare with `errors.Is` or classify with `errors.As`.
- Preserve typed errors. Do not wrap them in a way that destroys classification.
- Do not expose database, provider, stack, or secret internals in public API responses.

## Error Shape

Use sentinel errors for stable categories:

```go
var ErrOrderNotFound = errors.New("order not found")
```

Wrap with context at the point of failure:

```go
order, err := r.loadOrder(ctx, id)
if err != nil {
    return nil, fmt.Errorf("load order %s: %w", id, err)
}
```

Use typed errors when callers need structured data:

```go
type ValidationError struct {
    Field string
    Rule  string
}

func (e ValidationError) Error() string {
    return e.Field + " is invalid"
}
```

If the repo has a typed error package, prefer it for domain/application contracts and stable slugs/codes. Use raw `fmt.Errorf` mainly for private implementation context that will be translated before crossing a boundary.

## Layer Responsibilities

Infrastructure:

- Wrap driver/provider failures with operation context.
- Convert known dependency outcomes into domain/application errors when appropriate.
- Preserve original errors with `%w`.
- Add provider/status/operation context without leaking secrets.

Application:

- Add workflow context.
- Decide retry, compensation, idempotency, and transaction behavior.
- Avoid logging expected domain errors unless useful for audit.
- Publish or return domain/application events/errors without transport concerns.

Transport:

- Map errors to HTTP/gRPC/message responses.
- Log unexpected failures once with request/message metadata.
- Return safe, stable client messages.
- Translate validation, auth, not found, conflict, timeout, canceled, and unexpected errors consistently.

## HTTP And gRPC Mapping

Centralize mapping from internal errors to protocol responses:

```go
func statusFor(err error) int {
    switch {
    case errors.Is(err, ErrOrderNotFound):
        return http.StatusNotFound
    case errors.As(err, new(ValidationError)):
        return http.StatusBadRequest
    default:
        return http.StatusInternalServerError
    }
}
```

Do not scatter status-code decisions across handlers. Keep gRPC status mapping similarly centralized.

## Logging

Use structured logs. Include stable identifiers, not whole objects:

```go
logger.ErrorContext(ctx, "place order failed",
    "order_id", orderID,
    "customer_id", customerID,
    "error", err,
)
```

Log at boundaries:

- Unexpected application errors at the transport or worker boundary.
- External dependency failures where latency, provider, or status code matters.
- Background job and message handler start/finish/failure.
- Security/audit events required by the product.

Avoid:

- Logging secrets, tokens, raw payloads with personal data, or full SQL with arguments.
- Logging the same error repeatedly through every stack frame.
- Using logs as the only source for business metrics.
- Logging with the legacy `log` package in application code when the repo standard is `slog`.

Use context-aware logging when a request, job, message, or trace context is available.

## Metrics

Use metrics for aggregate behavior:

- Request count, latency, and error count by route/status.
- External dependency latency and failure count by provider/operation.
- Job duration, success/failure count, and queue lag.
- Domain counters for important business events.

Keep label cardinality bounded. Do not use user IDs, order IDs, email addresses, message UUIDs, or raw error strings as labels.

## Tracing

Propagate `context.Context` through every IO boundary. Use context-aware database, HTTP, broker, and logger calls.

Create spans around:

- Incoming requests and worker jobs.
- Database operations when not already instrumented.
- External API calls.
- Queue publish/consume.
- Long-running use case steps.

Attach useful attributes with bounded cardinality. Do not force tracing into pure domain code; prefer tracing at app/adapters/ports boundaries.

## Event-Driven Failures

For message handlers:

- Handler success should Ack; handler error should Nack or trigger retry according to the broker/router.
- Retry transient failures with bounded policy.
- Make handlers idempotent because duplicate delivery is normal.
- Log final failure with message UUID, event type, correlation ID, and aggregate ID.
- Send poison or permanently invalid messages to a dead-letter path when the infrastructure supports it.

## Done Criteria

- Callers can distinguish not-found, validation, conflict, permission, timeout, canceled, and unexpected errors.
- Unexpected errors are logged once with enough context to debug.
- Metrics answer "how often" and "how slow" without high-cardinality labels.
- Traces connect request, use case, database, broker, and external calls through `context.Context`.
- Public responses and message retry decisions are based on typed/classified errors, not string matching.
