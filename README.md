# Go Skills

[![skills.sh](https://skills.sh/b/emilwareus/go-skills)](https://skills.sh/emilwareus/go-skills)

Reusable Go engineering skills for agent-assisted work in Go services. They cover domain modeling, service boundaries, persistence, tests, event-driven workflows, and failure handling.

The skills are not project templates. They tell an agent what to inspect, where code belongs, what tradeoffs to name, and what to verify. They are informed by Go DDD and service architecture practices, including patterns discussed in the Three Dots Labs ecosystem. This project is not affiliated with Three Dots Labs.

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
| `go-domain-modeling` | Put business rules in entities, value objects, aggregates, and domain services. |
| `go-service-architecture` | Place code in ports, app, domain, adapters, and composition packages. |
| `go-persistence-transactions` | Design repositories, migrations, transactions, locking, idempotency, and outbox flows. |
| `go-testing` | Choose unit, application, adapter, component, integration, and event-driven test scopes. |
| `go-errors-observability` | Classify errors and add structured logs, metrics, traces, and protocol mappings. |
| `go-event-driven-watermill` | Build Watermill routers, CQRS buses, outbox/forwarders, and idempotent consumers. |

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
find skills -maxdepth 3 -name SKILL.md -print
npx skills add . --list
```

Keep each skill portable. Add examples only when they change a decision an agent would make. Mark project-specific conventions as optional.
