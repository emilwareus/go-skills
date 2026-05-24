---
name: go-errors-observability
description: Design Go error handling with wrapped errors, sentinel and typed error contracts, stable slug errors, boundary classification, and HTTP error mapping. Use for Go tasks involving public API errors, wrapping with %w, errors.Is/errors.As, or avoiding duplicate logs.
---

# Go Errors And Observability

Use this when a change touches error contracts, wrapping, logging boundaries, or protocol error mapping. Preserve error classification through the stack and translate errors once at the boundary. Keep the implementation small: wrapped errors, sentinel or typed contracts, slug responses, and one logging boundary.

## Error Contracts

Use stable sentinels when callers need category checks:

```go
var ErrTrainingNotFound = errors.New("training not found")
```

Use typed slug errors when public responses need stable machine-readable identifiers:

```go
return NewIncorrectInputError("date-from-after-date-to", "Date from after date to")
```

Public response slugs are API surface. Keep them stable and documented by tests. Use the human message for client-facing or diagnostic text, not for classification.

## Wrapping

Wrap where new operation context is added:

```go
training, err := r.loadTraining(ctx, id)
if err != nil {
    return nil, fmt.Errorf("load training %s: %w", id, err)
}
```

Boundary code should still classify wrapped errors with `errors.Is` and `errors.As`.

## HTTP Mapping

Centralize HTTP error mapping:

```go
func RespondWithSlugError(err error, w http.ResponseWriter, r *http.Request) {
    var slugErr SlugError
    if errors.As(err, &slugErr) {
        writeSlugResponse(slugErr, w, r)
        return
    }
    InternalError("internal-server-error", err, w, r)
}
```

Map expected input and authorization errors to stable client responses. Map unexpected failures to an internal slug and log the wrapped error with request metadata.

## Logging

Log unexpected failures once at the transport or worker boundary. Include stable identifiers such as request ID, route, message ID, aggregate ID, and slug. Keep secrets, raw payloads, tokens, and provider internals out of public responses and logs.

Expected user-input and authorization outcomes can return slug errors without error-level logs unless the product needs an audit trail.

## Anti-Patterns

- String-matching `err.Error()` in handlers or workers.
- Returning raw database, provider, stack, token, or secret-bearing error strings in public responses.
- Logging the same returned error in every repository, handler, adapter, and transport frame.
- Wrapping with `%v` or rebuilding errors in a way that breaks `errors.Is` and `errors.As`.
- Creating new public slugs casually; slug changes are API changes.
- Turning every error into a typed public error when only one internal caller needs a category.
- Swallowing unexpected errors at the boundary without logging enough request or message context.

## Examples

- [`examples/error_types.go`](examples/error_types.go) - slug-error type, constructors, sentinel error, and `%w` wrapping.
- [`examples/http_mapping.go`](examples/http_mapping.go) - `InternalError`, `Unauthorised`, `BadRequest`, and `RespondWithSlugError` helpers.

## Done Criteria

- Public responses use stable slugs.
- Wrapped errors preserve `errors.Is` and `errors.As` classification.
- Error-to-protocol mapping is centralized at the boundary.
- Unexpected errors are logged once with enough context to debug.
