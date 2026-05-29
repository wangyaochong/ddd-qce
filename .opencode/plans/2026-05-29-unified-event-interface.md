# Unified Event Interface Design

**Date:** 2026-05-29
**Priority:** P0
**Status:** Design Approved

## Problem

The framework maintains two event interfaces that create a dual-type system:

- `domain/event.Event` — 3 methods: `AggregateID()`, `CorrelationID()`, `CausationID()`
- `cqrs/event.Event` — embeds `domain/event.Event` + adds `OccurredAt() time.Time`

This split causes three categories of issues:

1. **Runtime panic risk**: Repository `Save()` performs `e.(event.Event)` hard type assertions (`infra/repository/pg/repository.go:201`, `exampleapp/.../repository.go:92`). A pure `domain/event.Event` implementation without `OccurredAt()` would panic.
2. **Conceptual inconsistency**: `When()` receives `domain/event.Event` but `When()` bodies access `e.OccurredAt()` via type switch to concrete types (which always embed `BaseEvent`). The interface doesn't declare what every consumer relies on.
3. **Learning cost**: Developers must understand which interface to use where, and when type conversions are needed.

## Decision

**Add `OccurredAt() time.Time` to `domain/event.Event`, delete `cqrs/event.Event` interface.**

Every domain event needs `OccurredAt()` — it is fundamental domain metadata, not an infrastructure concern. The `time` package is a Go standard library dependency, which does not violate `domain/event`'s "zero external dependency" principle (that principle prohibits third-party dependencies like `google/uuid`).

## Changes

### 1. `domain/event/event.go` — Extend Event interface

```go
type Event interface {
    AggregateID() string
    OccurredAt() time.Time    // NEW
    CorrelationID() string
    CausationID() string
}
```

### 2. `domain/event/doc.go` — Update documentation

Remove the note about "timing and serialization" being in `cqrs/event`. Update to reflect that `Event` now includes `OccurredAt()`.

### 3. `cqrs/event/event.go` — Delete Event interface, update types

- **Delete** the `Event` interface definition (lines 87-90)
- **Change** all references from `Event` (the local type) to `domainevent.Event`
- `BaseEvent` stays — it already implements `domainevent.Event` (all 4 methods)
- `EventHandler[T]` changes constraint to `T domainevent.Event`
- `GlobalEvent[T]` constraint unchanged (already `domainevent.Event`)
- `EventSourceStore[T]` constraint unchanged (already `domainevent.Event`)
- `ApplyCorrelation` and `RestoreBaseEvent` unchanged (already take `domainevent.Event`)

### 4. `cqrs/event/bus.go` — Update EventBus

```go
type EventBus interface {
    SubscribeHandler(handler any) error
    Publish(ctx context.Context, evt domainevent.Event) error
    SubscribedTypes() []string
}

func Dispatch[T domainevent.Event](ctx context.Context, bus EventBus, evt T) error {
    return bus.Publish(ctx, evt)
}
```

### 5. `infra/repository/pg/repository.go` — Eliminate type assertions

**Save() — before:**
```go
cqrsEvents := make([]event.Event, len(domainEvents))
for i, e := range domainEvents {
    cqrsEvents[i] = e.(event.Event)
}
if err := r.eventStore.Append(ctx, root.ID(), root.Version()-len(cqrsEvents), cqrsEvents); err != nil {
```

**Save() — after:**
```go
if err := r.eventStore.Append(ctx, root.ID(), root.Version()-len(domainEvents), domainEvents); err != nil {
```

**Load() — before:**
```go
if err := aggregate.LoadFromHistory(agg, domainevent.FromSlice(events)); err != nil {
```

**Load() — after:**
```go
if err := aggregate.LoadFromHistory(agg, events); err != nil {
```

### 6. `exampleapp/ddd/order/repository/repository.go` — Same patterns

Same assertion elimination in `Save()`, `FromSlice` elimination in `Load()`.

### 7. `cqrs/impl/pg/event_store.go` — Update assertions

`restoreBaseEvent` changes assertion target from `event.Event` (cqrs) to `domainevent.Event`.

### 8. `cqrs/impl/memory/event_bus.go` — Update type references

All `event.Event` references change to `domainevent.Event`.

### 9. `app/app.go` — No structural change

`EventBus` field type remains `event.EventBus` (which now publishes `domainevent.Event`).

### 10. `domain/aggregate/aggregate.go` — No change

Already uses `domain/event.Event`. With `OccurredAt()` added, `When()` can call `evt.OccurredAt()` directly.

### 11. Example app event definitions — No change needed

Events embed `BaseEvent` which already has `OccurredAt()`. They now satisfy the unified `domain/event.Event` interface.

## What Stays the Same

- `BaseEvent` stays in `cqrs/event` (depends on `trace` → `google/uuid`)
- `CorrelationSetter` / `Restorer` stay in `domain/event`
- `FromSlice[E Event]` stays in `domain/event`
- `ApplyCorrelation` / `RestoreBaseEvent` stay in `cqrs/event`
- `EventTypeOf` stays in `cqrs/event`

## Migration Impact

External projects need to:
1. Replace `cqrs/event.Event` with `domain/event.Event` in all type signatures
2. Remove `e.(event.Event)` type assertions
3. Remove `domainevent.FromSlice()` calls between identical types
4. Ensure custom event implementations provide `OccurredAt() time.Time`

## Verification

- All existing tests pass
- No remaining references to `cqrs/event.Event` interface
- `go vet` and `go build ./...` pass