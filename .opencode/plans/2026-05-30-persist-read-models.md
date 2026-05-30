# Persist Read Models Implementation Plan

**Goal:** Make business data (orders, inventory) survive restarts in PostgreSQL mode by replaying events on startup to rebuild in-memory read models.

**Architecture:** Events are already persisted to PG. On startup, we replay all stored events through the same domain logic to rebuild Order and Inventory read models. No schema changes needed — this is the classic CQRS event replay pattern.

**Scope:** Only `exampleapp/` code changes. No `core/` framework changes.

---

## Why Not PG Read-Model Tables?

| Approach | Pros | Cons |
|----------|------|------|
| **Event Replay (chosen)** | No new tables, leverages existing events, simple, no dual-write issues | Startup takes longer with many events |
| **PG read-model tables** | No replay time | New migrations, dual-write consistency, schema coupling |

For a test/demo app, replay is simpler and correct.

---

## Root Cause

When `DDD_STORE_TYPE=postgresql`, events are persisted but the in-memory `OrderRepository` and `Inventory` domain are not rebuilt from those events on startup. Solution: replay events on startup.

---

## Task 1: Create `replay.go` — Event Replay Engine

**Create:** `exampleapp/infrastructure/replay.go`

Core module that:
1. Loads all events from `EventSourceStore.LoadAll()`
2. Groups Order events by aggregate ID, replays each via `order.LoadFromHistory()`, saves to in-memory repo
3. Applies Inventory events (Reserve/Release) to the Inventory domain
4. Only runs when `store.DB != nil` (PostgreSQL mode)

Key interfaces already exist:
- `EventSourceStore.LoadAll(ctx, position, limit)` — loads events in batches
- `Order.LoadFromHistory(events)` — replays events on aggregate
- `OrderRepositoryAdapter.Save(ctx, order)` — saves to in-memory map
- `Inventory.Reserve/ProductID, qty)` / `Inventory.Release(ProductID, qty)` — updates stock

**Important:** Pass the raw `OrderRepository` (from `store.OrderRepo`), NOT the `OrderEventSourcedRepository`, to avoid re-appending events to the store.

---

## Task 2: Wire Replay Into App Startup

**Modify:** `exampleapp/infrastructure/wire.go`

Add replay call at end of `WireAppWithStore` when `store.DB != nil`:

```go
if store.DB != nil {
    if _, err := ReplayReadModels(context.Background(), store.EventStore, orderRepo, inventory); err != nil {
        log.Printf("[Replay] Warning: failed to replay read models: %v", err)
    }
}
```

Only runs in PG mode. If replay fails, log warning but don't crash the app.

---

## Task 3: Create Replay Integration Test

**Create:** `exampleapp/infrastructure/replay_test.go`

Test that:
1. Empty store → replay returns zero orders
2. Place order → replay into fresh repo → order appears with correct status
3. Inventory stock levels update correctly after replay

---

## Task 4: Existing Tests Still Pass

Run full test suite to ensure no regressions:
```bash
go test ./exampleapp/... -count=1 -timeout 180s
go test ./aspect/... ./job/... ./core/... -count=1
```

---

## Files Summary

| File | Action | Purpose |
|------|--------|---------|
| `exampleapp/infrastructure/replay.go` | Create | Replay engine |
| `exampleapp/infrastructure/wire.go` | Modify | Add replay call |
| `exampleapp/infrastructure/replay_test.go` | Create | Test replay logic |