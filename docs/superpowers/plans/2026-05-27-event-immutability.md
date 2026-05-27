# Event Immutability: Remove SetBaseEvent/SetCorrelation Exported Methods

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make `BaseEvent` truly immutable by removing exported setter methods (`SetBaseEvent`, `SetCorrelation`), replacing them with an unexported `restore` method only callable via reflection from the PG event store's `restoreBaseEvent`.

**Architecture:** Remove `SetBaseEvent` and `SetCorrelation` from `BaseEvent`. Add an unexported `restore` method that restores all 4 fields at once. Update PG `restoreBaseEvent` to call `restore` via reflection instead of the two separate setters. Update `AggregateRoot.Apply` to use a new unexported interface approach instead of the exported `SetCorrelation` type assertion. Update tests.

**Tech Stack:** Go, reflect, testing

---

### Task 1: Add unexported `restore` method to BaseEvent

**Files:**
- Modify: `cqrs/event/event.go`

- [ ] **Step 1: Add the `restore` method to BaseEvent**

Add this unexported method after the accessor methods (after line 49), before `EventTypeOf`:

```go
func (e *BaseEvent) restore(aggregateID string, occurredAt time.Time, correlationID, causationID string) {
	e.aggregateID = aggregateID
	e.occurredAt = occurredAt
	e.correlationID = correlationID
	e.causationID = causationID
}
```

- [ ] **Step 2: Run tests to verify no breakage yet**

Run: `go test ./cqrs/event/...`
Expected: PASS (old methods still exist)

---

### Task 2: Update PG `restoreBaseEvent` to call `restore` via reflection

**Files:**
- Modify: `cqrs/impl/pg/event_store.go`

- [ ] **Step 1: Replace `restoreBaseEvent` implementation**

Replace the entire `restoreBaseEvent` function (lines 218-244) with:

```go
func restoreBaseEvent(evt any, aggregateID string, occurredAt time.Time, correlationID string, causationID string) {
	v := reflect.ValueOf(evt)
	if v.Kind() == reflect.Ptr {
		v = v.Elem()
	}
	if v.Kind() != reflect.Struct {
		return
	}
	field := v.FieldByName("BaseEvent")
	if !field.IsValid() || field.Kind() != reflect.Struct {
		return
	}
	restorer := field.Addr().MethodByName("restore")
	if restorer.IsValid() {
		restorer.Call([]reflect.Value{
			reflect.ValueOf(aggregateID),
			reflect.ValueOf(occurredAt),
			reflect.ValueOf(correlationID),
			reflect.ValueOf(causationID),
		})
	}
}
```

- [ ] **Step 2: Run tests to verify `restore` path works**

Run: `go test ./cqrs/impl/pg/...`
Expected: PASS

---

### Task 3: Handle AggregateRoot.Apply's use of SetCorrelation

**Files:**
- Modify: `domain/aggregate/aggregate.go`
- Modify: `cqrs/event/event.go`

The `AggregateRoot.Apply` method at `domain/aggregate/aggregate.go:79-86` uses a type assertion to the exported `SetCorrelation` interface:
```go
if be, ok := evt.(interface{ SetCorrelation(string, string) }); ok {
    be.SetCorrelation(correlationID, causationID)
}
```

After removing `SetCorrelation`, this must change. Since `Apply` is called by domain code (not by infrastructure restoring from persistence), we need a different approach: instead of mutating the event, emit correlation metadata through the `Apply` method's own context or through a new mechanism.

**Approach:** Add an unexported `correlationSetter` interface in the `event` package, and export a helper function `SetCorrelationFromApply` that `AggregateRoot.Apply` can call. This keeps the setter unexported (external packages can't call it directly) while still allowing the aggregate package to set correlation through a controlled API.

Actually, the simpler approach: since `AggregateRoot.Apply` is the *only* legitimate caller (it sets correlation on a brand-new event during `Apply`, before the event is committed), we can:

1. Add an unexported `setCorrelation` method on `BaseEvent`
2. Export a function `ApplyCorrelation(evt Event, correlationID, causationID string)` from the `event` package that does the reflection-based call
3. `AggregateRoot.Apply` calls `event.ApplyCorrelation` instead of the type assertion

**Files:**
- Modify: `cqrs/event/event.go`
- Modify: `domain/aggregate/aggregate.go`

- [ ] **Step 1: Add `setCorrelation` unexported method and `ApplyCorrelation` helper to event.go**

After the `restore` method, add:

```go
func (e *BaseEvent) setCorrelation(correlationID, causationID string) {
	e.correlationID = correlationID
	e.causationID = causationID
}

func ApplyCorrelation(evt Event, correlationID, causationID string) {
	v := reflect.ValueOf(evt)
	if v.Kind() == reflect.Ptr {
		v = v.Elem()
	}
	if v.Kind() != reflect.Struct {
		return
	}
	field := v.FieldByName("BaseEvent")
	if !field.IsValid() || field.Kind() != reflect.Struct {
		return
	}
	setter := field.Addr().MethodByName("setCorrelation")
	if setter.IsValid() {
		setter.Call([]reflect.Value{
			reflect.ValueOf(correlationID),
			reflect.ValueOf(causationID),
		})
	}
}
```

- [ ] **Step 2: Update AggregateRoot.Apply to use `event.ApplyCorrelation`**

In `domain/aggregate/aggregate.go`, replace lines 80-86:

```go
		if evt.CorrelationID() == "" {
		if be, ok := evt.(interface{ SetCorrelation(string, string) }); ok {
			correlationID := trace.GetTraceID(ctx)
			causationID := trace.GetSpanID(ctx)
			be.SetCorrelation(correlationID, causationID)
		}
	}
```

with:

```go
	if evt.CorrelationID() == "" {
		event.ApplyCorrelation(evt, trace.GetTraceID(ctx), trace.GetSpanID(ctx))
	}
```

Also remove the `"github.com/ddd-qce/core/trace"` import from `aggregate.go` since `trace` is now used only as arguments to `ApplyCorrelation` — actually, `trace.GetTraceID(ctx)` and `trace.GetSpanID(ctx)` still need the import. Keep it.

- [ ] **Step 3: Run tests to verify**

Run: `go test ./domain/aggregate/... ./cqrs/event/...`
Expected: PASS

---

### Task 4: Remove exported `SetBaseEvent` and `SetCorrelation` from BaseEvent

**Files:**
- Modify: `cqrs/event/event.go`

- [ ] **Step 1: Delete `SetBaseEvent` and `SetCorrelation` methods**

Delete lines 51-59 from `cqrs/event/event.go`:

```go
func (e *BaseEvent) SetBaseEvent(aggregateID string, occurredAt time.Time) {
	e.aggregateID = aggregateID
	e.occurredAt = occurredAt
}

func (e *BaseEvent) SetCorrelation(correlationID, causationID string) {
	e.correlationID = correlationID
	e.causationID = causationID
}
```

- [ ] **Step 2: Verify nothing references the deleted methods**

Run: `grep -r 'SetBaseEvent\|SetCorrelation' --include='*.go' .`
Expected: No matches (except possibly in test files that will be updated in Task 5)

If there are remaining references, fix them before proceeding.

- [ ] **Step 3: Run full test suite**

Run: `go test ./...`
Expected: PASS

---

### Task 5: Update tests

**Files:**
- Modify: `cqrs/event/event_test.go`
- Modify: `cqrs/impl/pg/event_store_test.go`

- [ ] **Step 1: Delete `TestBaseEvent_SetBaseEvent` and `TestBaseEvent_SetCorrelation`**

Delete the two test functions from `cqrs/event/event_test.go` (lines 74-112):

```go
func TestBaseEvent_SetBaseEvent(t *testing.T) { ... }
func TestBaseEvent_SetCorrelation(t *testing.T) { ... }
```

- [ ] **Step 2: Add `TestRestoreBaseEvent` test**

Add a new test in `cqrs/event/event_test.go` that verifies the `restore` method works via reflection on an embedded `BaseEvent`:

```go
func TestRestoreBaseEvent(t *testing.T) {
	evt := &testEvent{}
	restoreBaseEvent(evt, "agg-1", time.Now(), "corr-1", "caus-1")

	if evt.AggregateID() != "agg-1" {
		t.Errorf("AggregateID() = %q, want %q", evt.AggregateID(), "agg-1")
	}
	if evt.CorrelationID() != "corr-1" {
		t.Errorf("CorrelationID() = %q, want %q", evt.CorrelationID(), "corr-1")
	}
	if evt.CausationID() != "caus-1" {
		t.Errorf("CausationID() = %q, want %q", evt.CausationID(), "caus-1")
	}
}
```

Note: `restoreBaseEvent` is unexported in the `pg` package, so this test must be in the `event` package. We need to either:
- Move the reflection logic into the `event` package as `RestoreBaseEvent` (exported helper), or
- Test it indirectly through the `ApplyCorrelation` function

Better approach: test `ApplyCorrelation` directly in `event_test.go`:

```go
func TestApplyCorrelation(t *testing.T) {
	evt := &testEvent{BaseEvent: NewBaseEvent("agg-1", time.Now())}
	if evt.CorrelationID() != "" {
		t.Errorf("CorrelationID() = %q, want empty before ApplyCorrelation", evt.CorrelationID())
	}

	ApplyCorrelation(evt, "corr-1", "caus-1")

	if evt.CorrelationID() != "corr-1" {
		t.Errorf("CorrelationID() = %q, want %q", evt.CorrelationID(), "corr-1")
	}
	if evt.CausationID() != "caus-1" {
		t.Errorf("CausationID() = %q, want %q", evt.CausationID(), "caus-1")
	}
}
```

- [ ] **Step 3: Add `TestApplyCorrelation_NoBaseEvent` for safety**

```go
type noBaseEventEvent struct {
	BaseEvent
	Value string
}

func (e *noBaseEventEvent) AggregateID() string   { return "hardcoded" }
func (e *noBaseEventEvent) OccurredAt() time.Time  { return time.Now() }
func (e *noBaseEventEvent) CorrelationID() string  { return "" }
func (e *noBaseEventEvent) CausationID() string    { return "" }

func TestApplyCorrelation_NilBaseEventField(t *testing.T) {
	evt := &testEvent{}
	ApplyCorrelation(evt, "corr-1", "caus-1")
	if evt.CorrelationID() != "corr-1" {
		t.Errorf("ApplyCorrelation should work on zero-value BaseEvent")
	}
}
```

- [ ] **Step 4: Add `AggregateID()`/`OccurredAt()` assertions to PG mock tests**

In `cqrs/impl/pg/event_store_test.go`, update `TestEventSourceStore_Load` to verify restored fields:

After `if len(events) != 1 { ... }`, add:

```go
if events[0].AggregateID() != "agg-1" {
	t.Errorf("AggregateID() = %q, want %q", events[0].AggregateID(), "agg-1")
}
```

- [ ] **Step 5: Run all tests**

Run: `go test ./...`
Expected: PASS

---

### Task 6: Final verification

- [ ] **Step 1: Run full test suite**

Run: `go test ./...`
Expected: All tests PASS

- [ ] **Step 2: Verify no exported setters remain**

Run: `grep -rn 'func.*BaseEvent.*Set' --include='*.go' cqrs/event/`
Expected: No matches (only unexported `restore` and `setCorrelation`)

- [ ] **Step 3: Verify external code cannot mutate events**

Confirm that from an external package, there is no way to call `restore`, `setCorrelation`, or any other method that mutates `BaseEvent` fields. The only mutation paths are:
1. `event.ApplyCorrelation` — sets correlation only, used by `AggregateRoot.Apply`
2. `restoreBaseEvent` (pg package, unexported) — used by PG store Load/LoadAll
3. `restore` (event package, unexported) — called only by `restoreBaseEvent` and `ApplyCorrelation` via reflection
