# Go Skills

[![skills.sh](https://skills.sh/b/emilwareus/go-skills)](https://skills.sh/emilwareus/go-skills)

Reusable Go engineering skills for agentic coding workflows. The skills focus on domain-first Go backend development: explicit business modeling, clear service boundaries, practical persistence, reliable tests, event-driven workflows, and diagnosable failures.

These skills are original packaging for reusable Go project guidance inspired by common Go DDD and service architecture practices, including patterns often discussed in the Three Dots Labs ecosystem. This project is not affiliated with Three Dots Labs.

## Install

Install all skills from this repository:

```bash
npx skills add emilwareus/go-skills
```

Install one skill:

```bash
npx skills add emilwareus/go-skills --skill go-domain-modeling
```

Install globally for supported agents:

```bash
npx skills add emilwareus/go-skills --global
```

## Skills

| Skill | Use when |
| --- | --- |
| `go-domain-modeling` | Modeling Go business domains with entities, value objects, aggregates, invariants, and domain services. |
| `go-service-architecture` | Structuring Go services with handlers, application use cases, dependency direction, adapters, and explicit wiring. |
| `go-persistence-transactions` | Designing repositories, database adapters, migrations, transaction boundaries, idempotency, locking, and outbox flows. |
| `go-testing` | Writing focused Go tests for domain logic, application services, handlers, repositories, concurrency, and integrations. |
| `go-errors-observability` | Handling Go errors, structured logging, metrics, tracing, and API error mapping. |
| `go-event-driven-watermill` | Building event-driven Go workflows with Watermill, CQRS buses, routers, middleware, outbox/forwarder, and idempotent consumers. |

## Repository Layout

```text
skills/
  go-domain-modeling/
    SKILL.md
  go-service-architecture/
    SKILL.md
  go-persistence-transactions/
    SKILL.md
  go-testing/
    SKILL.md
  go-errors-observability/
    SKILL.md
  go-event-driven-watermill/
    SKILL.md
```

Each skill follows the open Agent Skills format: a directory containing a `SKILL.md` file with YAML frontmatter containing `name` and `description`.

## Development

Validate basic package shape:

```bash
find skills -name SKILL.md -maxdepth 3 -print
npx skills add . --list
```

Keep each skill focused and portable across projects. Add examples only when they teach a reusable decision, and avoid project-specific conventions unless they are clearly marked as optional.
