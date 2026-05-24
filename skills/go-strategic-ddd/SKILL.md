---
name: go-strategic-ddd
description: Split Go business systems with strategic DDD boundaries, ubiquitous language, context maps, Event Storming outputs, direct ports, events, shared kernels, and anti-corruption adapters. Use for Go architecture tasks involving bounded contexts, modular monoliths, microservices, context coupling, service boundaries, or domain-driven decomposition.
---

# Go Strategic DDD

Use this when the change is about where a workflow belongs, how contexts talk, or whether a service/module split is correct. Start from the business flow, not from tables, endpoints, teams, or deployment units.

## Boundary Discovery

Use Event Storming-style inputs when they exist:

- Domain events: facts that already happened in the business.
- Commands: user/system intent that triggers a change.
- Actors: the person/system starting the command.
- Policies: reactions to events.
- Read models: information needed to make a decision.
- Aggregates: state that must stay consistent for a command.
- Pivotal events: points where responsibility naturally moves between contexts.

When no workshop artifact exists, reconstruct a small version from code and product language:

1. List the business events the workflow produces.
2. List the commands and actors that cause them.
3. Mark which rules must be checked immediately.
4. Mark which data can become consistent later.
5. Group events and rules that use the same language and change together.
6. Treat those groups as candidate bounded contexts.

## Bounded Context Rules

A bounded context owns one model and one vocabulary. Keep its code, public contracts, and persistence model aligned with that language.

Use these checks:

- Terms mean one thing inside the context.
- The context can change most of its behavior without editing another context's domain model.
- Other contexts talk to it through direct ports, public APIs, or events.
- Persistence details do not leak into another context.
- Deployment as one binary or many services is a later decision.

Prefer a modular monolith when boundaries are not yet proven. Keep the internal contracts strict enough that moving a context to a service later is mostly a wiring/deployment change.

## Context Integration

Choose one integration style per relationship:

- **Direct port**: synchronous in-process call when the caller needs an immediate answer and both contexts are in one deployable.
- **Public HTTP/gRPC API**: synchronous remote call when contexts are separately deployed and immediate response is required.
- **Domain event**: asynchronous reaction when the publisher only knows a fact happened.
- **Shared kernel**: small stable package for types that must be identical across contexts.
- **Anti-corruption adapter**: translator that protects one model from another context's concepts.

Define the port or event in the consuming context when it expresses the consumer's need. Keep the provider's domain types out of the consumer's domain package.

## Language Checks

Make names match the domain language:

- Commands are imperative: `CancelTraining`, `UsePoints`, `ScheduleTraining`.
- Events are past-tense facts: `TrainingCanceled`, `PointsUsed`, `HourMadeAvailable`.
- Aggregates are named after business concepts, not tables.
- Ports are named after the consuming use case need.
- DTOs are named as boundary payloads, not reused as domain objects.

If a name needs a suffix like `ForDiscount`, `API`, `DB`, or `Storage`, check whether it crosses a context boundary. The fix is usually an explicit translation object, not a shared struct.

## Split Review

Before adding a service or package split, answer:

- Which business capability owns this workflow?
- Which context's language does the new code use?
- Which data must be consistent immediately?
- Which data can be updated by event later?
- Which context owns the API/event contract?
- What breaks if this context is deployed separately later?

Before merging contexts, answer:

- Do the contexts change together in nearly every feature?
- Are calls between them synchronous and required for most operations?
- Is the model language actually the same?
- Would one module remove accidental network and orchestration complexity?

## Anti-Patterns

- Splitting services by database table, REST endpoint, CRUD resource, or team preference instead of business capability.
- Importing another context's domain/application package as a shortcut around a port or event.
- Reusing one struct for HTTP response, database document, event payload, and domain object.
- Treating microservices as a way to avoid modeling boundaries.
- Letting an event name describe what a downstream service should do.
- Creating shared kernel packages for unstable business concepts.
- Building an anti-corruption adapter that just passes foreign models through unchanged.

## References

- [DDD Knowledge Index](https://academy.threedots.tech/knowledge/ddd/)
- [Strategic Domain Design Knowledge Index](https://academy.threedots.tech/knowledge/strategic-domain-design/)
- [Anti Corruption Layer Knowledge Index](https://academy.threedots.tech/knowledge/anti-corruption-layer/)
- [Shared Kernel Knowledge Index](https://academy.threedots.tech/knowledge/shared-kernel/)
- [Software Dark Ages](https://threedots.tech/post/software-dark-ages/)
- [When using Microservices or Modular Monolith in Go can be just a detail?](https://threedots.tech/post/microservices-or-monolith-its-detail/)
- [When to avoid DRY in Go](https://threedots.tech/post/things-to-know-about-dry/)
- [Combining DDD, CQRS, and Clean Architecture in Go](https://threedots.tech/post/ddd-cqrs-clean-architecture-combined/)

## Done Criteria

- Boundaries follow business language and consistency needs.
- Cross-context communication uses ports, APIs, events, shared kernel, or anti-corruption adapters deliberately.
- Domain packages do not import another context's domain as an implementation shortcut.
- Event names and command names describe the publisher/caller's model.
- The split can be explained without mentioning tables, endpoints, or deployment tooling first.
