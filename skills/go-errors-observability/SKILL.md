---
name: go-errors-observability
description: Design Go error handling with wrapped errors, sentinel and typed error contracts, stable slug errors, boundary classification, and HTTP error mapping. Use for Go tasks involving error packages, public API error responses, wrapping with %w, errors.Is/errors.As, or avoiding duplicate logs.
---

# Go Errors And Observability

Use this when a Go change touches error contracts, wrapping, logging boundaries, or protocol error mapping. This skill is a small authored synthesis. It is not backed by a dedicated Three Dots Labs observability article; the examples use Wild Workouts' slug-error and `httperr` concepts.

## Error Principles

- Return errors; do not log and return the same error at every layer.
- Wrap implementation failures with operation context using `%w`.
- Use sentinel errors when callers only need a stable category.
- Use typed errors when the boundary needs structured information.
- Use stable slugs for public API errors.
- Classify errors at the transport or worker boundary.
- Do not expose database, provider, stack, or secret internals in public responses.

## Slug Pattern

Wild Workouts uses slug errors for application/domain failures that cross into HTTP:

```go
return errors.NewIncorrectInputError("date-from-after-date-to", "Date from after date to")
```

Use the slug as API surface and the human message as diagnostic text. At the boundary, map the typed slug error to a response:

```go
httperr.RespondWithSlugError(err, w, r)
httperr.InternalError("cannot-get-user", err, w, r)
httperr.Unauthorised("invalid-role", nil, w, r)
```

Do not string-match `err.Error()` in handlers.

## Wrapping

Wrap where context is added:

```go
training, err := r.loadTraining(ctx, id)
if err != nil {
    return nil, fmt.Errorf("load training %s: %w", id, err)
}
```

The boundary should still be able to use `errors.Is` for sentinels and `errors.As` for typed slug errors after wrapping.

## Logging

Log unexpected failures once at the transport or worker boundary, with request/message metadata and the final wrapped error. Do not add another error log in every repository, handler, and adapter frame.

Expected user-input and authorization outcomes can be returned as slug errors without error-level logs unless the product needs an audit trail.

## Examples

Annotated reference implementations live in `examples/`:

- [`examples/error_types.go`](examples/error_types.go) — the slug-error shape, `NewIncorrectInputError`, `NewAuthorizationError`, a sentinel error, and `%w` wrapping.
- [`examples/http_mapping.go`](examples/http_mapping.go) — Wild Workouts-style `InternalError`, `Unauthorised`, `BadRequest`, and `RespondWithSlugError` helpers.

## Done Criteria

- Callers can distinguish expected user/input/auth failures from unexpected failures.
- Public responses use stable slugs, not raw internal error strings.
- Wrapped errors preserve `errors.Is` / `errors.As` classification.
- Unexpected errors are logged once at the boundary with enough context to debug.
