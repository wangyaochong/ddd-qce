# API Stability

## Stable Public API

These packages are part of the stable public API:

| Package | Purpose |
|---------|---------|
| `app` | Application bootstrap and lifecycle |
| `aspect` | AOP aspect chain |
| `aspect/builtin` | Built-in aspects (logging, tracing, metrics, transaction, persistence) |
| `config` | Configuration loading |
| `cqrs/command` | Command interfaces and dispatch |
| `cqrs/query` | Query interfaces and dispatch |
| `cqrs/event` | Event interfaces and dispatch |
| `domain/aggregate` | AggregateRoot |
| `domain/entity` | Entity, AuditableEntity, SoftDeletableEntity |
| `domain/event` | Event interface |
| `domain/repository` | Repository interfaces |
| `domain/valueobject` | ValueObject |
| `error` | Domain error types |
| `infra` | Backend abstraction |
| `infra/repository` | Repository error types |
| `job/core` | Job interfaces |
| `pg` | PostgreSQL utilities |
| `trace` | Tracing interfaces |

## Stable Implementations

These implementations are stable and can be used directly:

| Package | Purpose |
|---------|---------|
| `cqrs/impl/memory` | In-memory CQRS buses |
| `infra/repository/memory` | In-memory repositories |
| `job/memory` | In-memory job system |
| `trace` | In-memory trace store |

## Experimental

These packages may change in future versions:

| Package | Purpose |
|---------|---------|
| `observability` | Dashboard and schema viewer |
| `lint/*` | DDD static analysis (separate module) |
